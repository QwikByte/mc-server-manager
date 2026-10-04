// Package plugin installs plugins and mods from Modrinth on servers, and lists, updates
// and removes the installed ones. The master downloads each file once, checks its hash
// and streams it to the agents, which write it into the plugin folder of the server.
package plugin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modrinth"
)

const (
	chunkSize = 256 << 10
	// maxProjects limits the projects installed at once, including required ones.
	maxProjects = 50
	iconRoute   = "/api/plugins/icons/"
)

var errVanilla = httpapi.Errorf(http.StatusConflict, "Vanilla servers can't load plugins or mods.")

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Ref points to a server on a node.
type Ref struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

// Project is a Modrinth project as the panel shows it.
type Project struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Icon is the path of the project icon in the panel, which proxies Modrinth's CDN.
	Icon string `json:"icon,omitempty"`
}

type Service struct {
	nodes    Nodes
	modrinth *modrinth.Client
}

func NewService(nodes Nodes, client *modrinth.Client) *Service {
	return &Service{nodes: nodes, modrinth: client}
}

// Projects looks up projects on Modrinth; unknown ones are left out.
func (s *Service) Projects(ctx context.Context, ids []string) ([]modrinth.Project, error) {
	return s.modrinth.Projects(ctx, ids)
}

// Describe returns a project as the panel shows it.
func (s *Service) Describe(p modrinth.Project) Project {
	return Project{ID: p.ID, Slug: p.Slug, Title: p.Title, Icon: s.icon(p.IconURL)}
}

func (s *Service) icon(url string) string {
	if name := s.modrinth.IconName(url); name != "" {
		return iconRoute + name
	}
	return ""
}

// Plugin is a plugin file on a server, with its Modrinth project if it comes from there.
type Plugin struct {
	FileName string   `json:"fileName"`
	Size     int64    `json:"size"`
	Project  *Project `json:"project,omitempty"`
	Version  string   `json:"version,omitempty"`
	// VersionID is the ID of the version on Modrinth.
	VersionID string `json:"versionId,omitempty"`
	// Update is a newer release of the project for the server.
	Update string `json:"update,omitempty"`
}

type Listing struct {
	Folder  string   `json:"folder"` // plugins or mods
	Plugins []Plugin `json:"plugins"`
	// CatalogueError tells why the plugins couldn't be looked up on Modrinth.
	CatalogueError string `json:"catalogueError,omitempty"`
}

// List returns the plugins of a server. Files from Modrinth are identified by their hash.
func (s *Service) List(ctx context.Context, ref Ref) (Listing, error) {
	conn, srv, err := s.server(ctx, ref)
	if err != nil {
		return Listing{}, err
	}
	res, err := noryxv1.NewPluginServiceClient(conn).ListPlugins(ctx, &noryxv1.ListPluginsRequest{ServerId: ref.ServerID})
	if err != nil {
		return Listing{}, err
	}
	l := Listing{Folder: res.GetFolder(), Plugins: []Plugin{}}
	for _, p := range res.GetPlugins() {
		l.Plugins = append(l.Plugins, Plugin{FileName: p.GetFileName(), Size: p.GetSize()})
	}
	if len(l.Plugins) > 0 {
		if err := s.describe(ctx, srv, res.GetPlugins(), l.Plugins); err != nil {
			l.CatalogueError = message(err)
		}
	}
	return l, nil
}

// describe adds the Modrinth project, version and available update to each plugin.
func (s *Service) describe(ctx context.Context, srv *noryxv1.Server, files []*noryxv1.PluginFile, plugins []Plugin) error {
	hashes := make([]string, len(files))
	for i, f := range files {
		hashes[i] = f.GetSha512()
	}
	t, err := s.target(ctx, srv.GetType(), srv.GetVersion())
	if err != nil {
		return err
	}
	versions, err := s.modrinth.VersionsByHash(ctx, hashes)
	if err != nil {
		return err
	}
	updates, err := s.modrinth.Updates(ctx, hashes, t.loaders, t.gameVersion)
	if err != nil {
		return err
	}
	var ids []string
	for _, v := range versions {
		ids = append(ids, v.ProjectID)
	}
	projects, err := s.modrinth.Projects(ctx, slices.Compact(slices.Sorted(slices.Values(ids))))
	if err != nil {
		return err
	}
	for i, hash := range hashes {
		v, known := versions[hash]
		j := slices.IndexFunc(projects, func(p modrinth.Project) bool { return p.ID == v.ProjectID })
		if !known || j < 0 {
			continue
		}
		project := s.Describe(projects[j])
		plugins[i].Project, plugins[i].Version, plugins[i].VersionID = &project, v.VersionNumber, v.ID
		if u, ok := updates[hash]; ok && u.ID != v.ID && u.Published.After(v.Published) {
			plugins[i].Update = u.VersionNumber
		}
	}
	return nil
}

// Installed is a plugin that was installed on a server.
type Installed struct {
	ProjectID string `json:"projectId"`
	FileName  string `json:"fileName"`
	Version   string `json:"version"`
}

// Result tells what was installed on a server, and why the rest wasn't.
type Result struct {
	Ref
	Installed []Installed `json:"installed"`
	Error     string      `json:"error,omitempty"`
}

// Install installs the newest suitable release of each project on the given servers,
// or the version chosen for it by project ID, together with the projects it requires.
// Installed projects are updated, but projects that are only required are left as they
// are.
func (s *Service) Install(ctx context.Context, projects []string, chosen map[string]string, servers []Ref) []Result {
	run := &installation{Service: s, chosen: chosen}
	results := make([]Result, len(servers))
	var wg sync.WaitGroup
	for i, ref := range servers {
		wg.Go(func() {
			installed, err := run.install(ctx, ref, projects)
			results[i] = Result{Ref: ref, Installed: installed}
			if err != nil {
				results[i].Error = message(err)
			}
		})
	}
	wg.Wait()
	return results
}

// installation shares lookups and downloads between the servers of one Install call.
type installation struct {
	*Service
	chosen    map[string]string // version IDs by project ID
	versions  memo[[]modrinth.Version]
	downloads memo[[]byte]
	titles    memo[string]
}

func (r *installation) install(ctx context.Context, ref Ref, projects []string) ([]Installed, error) {
	conn, srv, err := r.server(ctx, ref)
	if err != nil {
		return nil, err
	}
	t, err := r.target(ctx, srv.GetType(), srv.GetVersion())
	if err != nil {
		return nil, err
	}
	plugins := noryxv1.NewPluginServiceClient(conn)
	present, err := r.present(ctx, plugins, ref.ServerID)
	if err != nil {
		return nil, err
	}
	versions, err := r.resolve(ctx, t, projects, present)
	if err != nil {
		return nil, err
	}
	installed := []Installed{}
	for _, v := range versions {
		f, _ := v.File()
		if present[v.ProjectID] != f.Filename { // otherwise this version is installed already
			data, err := r.downloads.get(f.URL, func() ([]byte, error) { return r.modrinth.Download(ctx, f) })
			if err != nil {
				return installed, err
			}
			header := &noryxv1.InstallPluginHeader{ServerId: ref.ServerID, FileName: f.Filename, Replaces: present[v.ProjectID]}
			if _, err := send(ctx, plugins, header, bytes.NewReader(data)); err != nil {
				return installed, err
			}
		}
		installed = append(installed, Installed{ProjectID: v.ProjectID, FileName: f.Filename, Version: v.VersionNumber})
	}
	return installed, nil
}

// present returns the file of each Modrinth project installed on a server.
func (r *installation) present(ctx context.Context, c noryxv1.PluginServiceClient, serverID string) (map[string]string, error) {
	res, err := c.ListPlugins(ctx, &noryxv1.ListPluginsRequest{ServerId: serverID})
	if err != nil || len(res.GetPlugins()) == 0 {
		return map[string]string{}, err
	}
	files := map[string]string{}
	for _, p := range res.GetPlugins() {
		files[p.GetSha512()] = p.GetFileName()
	}
	versions, err := r.modrinth.VersionsByHash(ctx, slices.Collect(maps.Keys(files)))
	present := map[string]string{}
	for hash, v := range versions {
		present[v.ProjectID] = files[hash]
	}
	return present, err
}

// resolve picks the version of each project and of the projects they require.
func (r *installation) resolve(ctx context.Context, t target, projects []string, present map[string]string) ([]modrinth.Version, error) {
	var picked []modrinth.Version
	queue, seen := slices.Clone(projects), map[string]bool{}
	for ; len(queue) > 0; queue = queue[1:] {
		id := queue[0]
		_, installed := present[id]
		if seen[id] || installed && !slices.Contains(projects, id) {
			continue
		}
		seen[id] = true
		if len(seen) > maxProjects {
			return nil, httpapi.Errorf(http.StatusBadRequest, "Choose fewer projects; they require more than %d projects together.", maxProjects)
		}
		v, err := r.pick(ctx, id, t)
		if err != nil {
			return nil, err
		}
		for _, d := range v.Dependencies {
			if d.Type == "required" && d.ProjectID != "" {
				queue = append(queue, d.ProjectID)
			}
		}
		picked = append(picked, v)
	}
	return picked, nil
}

// pick returns the version chosen for a project if it suits the server, else the
// newest release of the project for the server, or else its newest version.
func (r *installation) pick(ctx context.Context, project string, t target) (modrinth.Version, error) {
	versions, err := r.compatible(ctx, project, t)
	if err != nil {
		return modrinth.Version{}, err
	}
	i := max(0, slices.IndexFunc(versions, func(v modrinth.Version) bool { return v.VersionType == "release" }))
	chosen := r.chosen[project]
	if chosen != "" {
		i = slices.IndexFunc(versions, func(v modrinth.Version) bool { return v.ID == chosen })
	}
	if i >= 0 && i < len(versions) && len(versions[i].Files) > 0 {
		return versions[i], nil
	}
	title, _ := r.titles.get(project, func() (string, error) {
		projects, err := r.modrinth.Projects(ctx, []string{project})
		if err != nil || len(projects) == 0 {
			return project, err
		}
		return projects[0].Title, nil
	})
	if chosen != "" {
		return modrinth.Version{}, httpapi.Errorf(http.StatusConflict, "The chosen version of %s doesn't run on %s.", title, t)
	}
	return modrinth.Version{}, httpapi.Errorf(http.StatusConflict, "%s has no version for %s.", title, t)
}

// compatible returns the versions of a project that run on a server, the newest first.
func (r *installation) compatible(ctx context.Context, project string, t target) ([]modrinth.Version, error) {
	key := project + "|" + strings.Join(t.loaders, ",") + "|" + t.gameVersion
	return r.versions.get(key, func() ([]modrinth.Version, error) {
		return r.modrinth.Versions(ctx, project, t.loaders, t.gameVersion)
	})
}

// Version is a version of a project as the panel shows it.
type Version struct {
	ID        string    `json:"id"`
	Number    string    `json:"number"`
	Channel   string    `json:"channel"` // release, beta or alpha
	Published time.Time `json:"published"`
}

// Versions returns the versions of a project that run on servers of a type and
// Minecraft version, the newest first.
func (s *Service) Versions(ctx context.Context, project string, typ noryxv1.ServerType, gameVersion string) ([]Version, error) {
	t, err := s.target(ctx, typ, gameVersion)
	if err != nil {
		return nil, err
	}
	found, err := s.modrinth.Versions(ctx, project, t.loaders, t.gameVersion)
	versions := make([]Version, 0, len(found))
	for _, v := range found {
		if len(v.Files) > 0 {
			versions = append(versions, Version{ID: v.ID, Number: v.VersionNumber, Channel: v.VersionType, Published: v.Published})
		}
	}
	return versions, err
}

// target is what a plugin must support to run on a server.
type target struct {
	loaders     []string
	gameVersion string // empty for proxies, which run with any version
}

func (t target) String() string {
	name := strings.ToUpper(t.loaders[0][:1]) + t.loaders[0][1:]
	return strings.TrimSpace(name + " " + t.gameVersion)
}

// target returns what plugins must support to run on servers of a type and Minecraft
// version.
func (s *Service) target(ctx context.Context, typ noryxv1.ServerType, gameVersion string) (target, error) {
	t := target{loaders: modrinth.Loaders(typ)}
	switch {
	case len(t.loaders) == 0:
		return t, errVanilla
	case typ.Proxy():
		return t, nil
	case gameVersion == "LATEST":
		var err error
		t.gameVersion, err = s.modrinth.LatestRelease(ctx)
		return t, err
	}
	t.gameVersion = gameVersion
	return t, nil
}

// Ensure installs the newest suitable release of a project on a server, together with the
// projects it requires, unless the project is installed already.
func (s *Service) Ensure(ctx context.Context, ref Ref, project string) error {
	conn, _, err := s.server(ctx, ref)
	if err != nil {
		return err
	}
	run := &installation{Service: s}
	present, err := run.present(ctx, noryxv1.NewPluginServiceClient(conn), ref.ServerID)
	if _, ok := present[project]; ok || err != nil {
		return err
	}
	_, err = run.install(ctx, ref, []string{project})
	return err
}

// Uninstall removes the file of a project from a server, if it has one.
func (s *Service) Uninstall(ctx context.Context, ref Ref, project string) error {
	conn, _, err := s.server(ctx, ref)
	if err != nil {
		return err
	}
	present, err := (&installation{Service: s}).present(ctx, noryxv1.NewPluginServiceClient(conn), ref.ServerID)
	if file, ok := present[project]; ok && err == nil {
		return s.Remove(ctx, ref, file)
	}
	return err
}

// Remove deletes a plugin file from a server.
func (s *Service) Remove(ctx context.Context, ref Ref, fileName string) error {
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return err
	}
	_, err = noryxv1.NewPluginServiceClient(conn).RemovePlugin(ctx, &noryxv1.RemovePluginRequest{ServerId: ref.ServerID, FileName: fileName})
	return err
}

// Upload installs a plugin file of the administrator on a server.
func (s *Service) Upload(ctx context.Context, ref Ref, fileName string, content io.Reader) (Plugin, error) {
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return Plugin{}, err
	}
	header := &noryxv1.InstallPluginHeader{ServerId: ref.ServerID, FileName: fileName}
	p, err := send(ctx, noryxv1.NewPluginServiceClient(conn), header, content)
	return Plugin{FileName: p.GetFileName(), Size: p.GetSize()}, err
}

// send streams a plugin file to the agent.
func send(ctx context.Context, c noryxv1.PluginServiceClient, header *noryxv1.InstallPluginHeader, content io.Reader) (*noryxv1.PluginFile, error) {
	stream, err := c.InstallPlugin(ctx)
	if err != nil {
		return nil, err
	}
	err = stream.Send(&noryxv1.InstallPluginRequest{Content: &noryxv1.InstallPluginRequest_Header{Header: header}})
	buf := make([]byte, chunkSize)
	for err == nil {
		n, readErr := io.ReadFull(content, buf)
		if n > 0 {
			err = stream.Send(&noryxv1.InstallPluginRequest{Content: &noryxv1.InstallPluginRequest_Data{Data: buf[:n]}})
		}
		if err == nil {
			err = readErr
		}
	}
	// EOF of the content: everything was sent. EOF of Send: the agent ended the stream,
	// its reply tells why.
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	res, err := stream.CloseAndRecv()
	return res.GetPlugin(), err
}

// server looks up a server on its node.
func (s *Service) server(ctx context.Context, ref Ref) (grpc.ClientConnInterface, *noryxv1.Server, error) {
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return nil, nil, err
	}
	res, err := noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(res.GetServers(), func(srv *noryxv1.Server) bool { return srv.GetId() == ref.ServerID })
	if i < 0 {
		return nil, nil, httpapi.Errorf(http.StatusNotFound, "Server not found.")
	}
	return conn, res.GetServers()[i], nil
}

// message returns what administrators are told about an error.
func message(err error) string {
	var apiErr *httpapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Message
	}
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	slog.Error("Plugin operation failed", logging.Plugins, "err", err)
	return "internal error"
}

// memo runs a function once per key and shares its result, also between goroutines.
type memo[T any] struct {
	mu    sync.Mutex
	funcs map[string]func() (T, error)
}

func (m *memo[T]) get(key string, fn func() (T, error)) (T, error) {
	m.mu.Lock()
	f, ok := m.funcs[key]
	if !ok {
		f = sync.OnceValues(fn)
		if m.funcs == nil {
			m.funcs = map[string]func() (T, error){}
		}
		m.funcs[key] = f
	}
	m.mu.Unlock()
	return f()
}
