// Package server exposes the Minecraft servers of all nodes to the panel. Agents are
// the source of truth: every request is forwarded to the agent of the addressed node.
package server

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modpack"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

const (
	probeTimeout = 3 * time.Second // for listing the servers of all nodes
	queryTimeout = 10 * time.Second
	// actionTimeout covers a graceful stop, which may take the longest stop timeout.
	actionTimeout = 2*time.Minute + noryxv1.MaxStopTimeout
	createTimeout = 10 * time.Minute // includes pulling the server image
	// changeTimeout covers pulling the server image and recreating a running server's container.
	changeTimeout = createTimeout + noryxv1.MaxStopTimeout
)

// Nodes provides the nodes and connections to their agents.
type Nodes interface {
	List(ctx context.Context) ([]node.Node, error)
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Networks keep servers in networks: one can't be deleted while it is in a network, and a
// network follows a server that moves, if it may.
type Networks interface {
	CheckRemovable(ctx context.Context, nodeID, serverID string) error
	// CheckMove fails if a server can't move to another node; unreachable confirms a move to
	// a node that doesn't reach the datastores of its network.
	CheckMove(ctx context.Context, serverID, from, to string, unreachable bool) error
	Move(ctx context.Context, serverID, from, to string) error
	// Reapply configures the network of a server again, e.g. as its port changed.
	Reapply(ctx context.Context, serverID string) error
}

// Tags label servers, e.g. lobby, and notes describe them, so that the panel finds and groups
// them.
type Tags interface {
	All(ctx context.Context) (map[tag.Server][]string, error)
	Change(ctx context.Context, servers []tag.Server, add, remove []string) error
	Notes(ctx context.Context) (map[tag.Server]string, error)
	SetNotes(ctx context.Context, srv tag.Server, notes string) error
	// Copy gives a copy of a server the tags and notes of the original.
	Copy(ctx context.Context, from, to tag.Server) error
}

// FileSets put shared files on the servers of tags. Servers that lose a tag lose the files
// with secrets of its sets.
type FileSets interface {
	Left(ctx context.Context, servers []tag.Server)
}

// Plugins installs plugins and mods, e.g. those of a template on a new server.
type Plugins interface {
	InstallOn(ctx context.Context, projects []string, nodeID, serverID string) error
}

// Modpacks installs Modrinth modpacks on new servers.
type Modpacks interface {
	Resolve(ctx context.Context, project, version string) (*modpack.Pack, error)
	Install(ctx context.Context, nodeID, serverID string, p *modpack.Pack) error
}

// References refer to servers, e.g. the targets of backup jobs and the scopes of groups.
type References interface {
	// Forget forgets a deleted server.
	Forget(ctx context.Context, nodeID, serverID string) error
	// Move refers to a server that moved to another node at its new place.
	Move(ctx context.Context, serverID, from, to string) error
}

type Handler struct {
	nodes    Nodes
	networks Networks
	tags     Tags
	plugins  Plugins
	modpacks Modpacks
	ops      *operation.Operations
	moves    *Moves
	sets     FileSets
	refs     []References
	reserved reservations
}

func NewHandler(nodes Nodes, networks Networks, tags Tags, plugins Plugins, modpacks Modpacks, ops *operation.Operations, moves *Moves, sets FileSets, refs ...References) *Handler {
	return &Handler{nodes: nodes, networks: networks, tags: tags, plugins: plugins, modpacks: modpacks, ops: ops, moves: moves, sets: sets, refs: refs}
}

// What creating a server and copying one need, also to cancel them. The copy contains all
// files of the server.
var (
	createNeed    = access.OnNode(access.ServersCreate, "node")
	duplicateNeed = access.All(createNeed, access.OnServer(access.FilesRead))
)

// Register adds the routes. The lists only contain the servers the user may see.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/servers", access.SignedIn, h.listAll)
	mux.Handle("GET /api/nodes/{node}/servers", access.SignedIn, h.list)
	// Bulk requests check the permission for each server they name.
	mux.Handle("POST /api/servers/actions", access.SignedIn, h.bulk)
	mux.Handle("POST /api/servers/tags", access.SignedIn, h.changeTags)
	mux.Handle("POST /api/nodes/{node}/servers", createNeed, h.create)
	mux.Handle("POST /api/nodes/{node}/servers/{id}/start", access.OnServer(access.ServersStart), h.startServer)
	for _, action := range []string{"stop", "restart"} {
		mux.Handle("POST /api/nodes/{node}/servers/{id}/"+action, access.OnServer(serverActions[action].need), h.power(action))
	}
	mux.Handle("DELETE /api/nodes/{node}/servers/{id}", access.OnServer(access.ServersDelete), h.delete)
	mux.Handle("PUT /api/nodes/{node}/servers/{id}", access.OnServer(access.ServersSettings), h.update)
	mux.Handle("POST /api/nodes/{node}/servers/{id}/update-image", access.OnServer(access.ServersSettings), h.updateImage)
	mux.Handle("PUT /api/nodes/{node}/servers/{id}/notes", access.OnServer(access.ServersSettings), h.setNotes)
	// The copy contains all files of the server.
	mux.Handle("POST /api/nodes/{node}/servers/{id}/duplicate", duplicateNeed, h.duplicate)
	// Moving takes the server away from where it is and copies all its files.
	mux.Handle("POST /api/nodes/{node}/servers/{id}/move", access.All(access.OnServer(access.ServersDelete), access.OnServer(access.FilesRead)), h.move)
	mux.Handle("GET /api/moves", access.SignedIn, h.listMoves)
	mux.Handle("GET /api/nodes/{node}/servers/{id}/logs", access.OnServer(access.ConsoleView), h.logs)
	mux.Handle("GET /api/nodes/{node}/servers/{id}/logs/earlier", access.OnServer(access.ConsoleView), h.earlier)
	mux.Handle("POST /api/nodes/{node}/servers/{id}/command", access.OnServer(access.ConsoleCommands), h.command)
}

type view struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Version  string `json:"version"`
	MemoryMB uint32 `json:"memoryMb"`
	// MemoryLimitMB is the limit of the server's container, which the memory limit of its node counts.
	MemoryLimitMB int64  `json:"memoryLimitMb"`
	Port          uint32 `json:"port"`
	State         string `json:"state"`
	Storage       string `json:"storage"`
	// Crashes since the server was last started, while it crashes or after it stopped
	// because of a crash, and the exit code of the latest one, 0 if unknown.
	Crashes  uint32 `json:"crashes"`
	ExitCode int32  `json:"exitCode"`
	// BedrockPort is the UDP port at which Bedrock players join a proxy, which its network sets.
	BedrockPort uint32 `json:"bedrockPort,omitempty"`
	// Overlay tells that the node publishes the port only in the private network of the
	// nodes, for the node of the server's proxy.
	Overlay bool `json:"overlay,omitempty"`
	// Unhealthy tells that a running server's health check fails, e.g. as it hangs.
	Unhealthy bool `json:"unhealthy,omitempty"`
	// RefusedJVMOptions are JVM options set before the agent refused them, which the server
	// still starts with until they are removed.
	RefusedJVMOptions []string `json:"refusedJvmOptions,omitempty"`
	settings
}

// settings are the settings of a server that can be changed after it was created.
type settings struct {
	Java          string   `json:"java"`
	LoaderVersion string   `json:"loaderVersion"`
	RestartPolicy string   `json:"restartPolicy"`
	AikarFlags    bool     `json:"aikarFlags"`
	JVMOptions    []string `json:"jvmOptions"`
	CPULimit      float64  `json:"cpuLimit"` // in cores, 0 means no limit
	// StopTimeout is how many seconds the server gets to stop gracefully; 0 means the default.
	StopTimeout uint32 `json:"stopTimeout"`
	// TimeZone is an IANA time zone such as Europe/Berlin; empty means UTC.
	TimeZone string `json:"timeZone"`
}

// check validates the settings that the agent can't, and converts the restart policy
// and CPU limit; an empty restart policy means the default.
func (s settings) check() (noryxv1.RestartPolicy, uint32, error) {
	policy := noryxv1.ParseRestartPolicy(s.RestartPolicy)
	switch {
	case s.CPULimit < 0 || s.CPULimit > 1024:
		return policy, 0, httpapi.Errorf(http.StatusBadRequest, "Enter a CPU limit in cores, or 0 for no limit.")
	case policy == noryxv1.RestartPolicy_RESTART_POLICY_UNSPECIFIED && s.RestartPolicy != "":
		return policy, 0, httpapi.Errorf(http.StatusBadRequest, "Choose when the server starts on its own.")
	}
	return policy, uint32(math.Round(s.CPULimit * 1000)), nil
}

func toView(s *noryxv1.Server) view {
	return view{
		ID: s.GetId(), Name: s.GetName(), Version: s.GetVersion(), MemoryMB: s.GetMemoryMb(), MemoryLimitMB: noryxv1.ContainerMemoryMB(s.GetMemoryMb()), Port: s.GetPort(),
		Type: s.GetType().Slug(), State: s.GetState().Slug(), Storage: s.GetStorage(), Crashes: s.GetCrashes(), ExitCode: s.GetExitCode(),
		BedrockPort: s.GetBedrockPort(), RefusedJVMOptions: s.GetRefusedJvmOptions(), Overlay: s.GetOverlay(), Unhealthy: s.GetUnhealthy(),
		settings: settings{
			Java: s.GetJava(), LoaderVersion: s.GetLoaderVersion(), RestartPolicy: s.GetRestartPolicy().Slug(), AikarFlags: s.GetAikarFlags(),
			JVMOptions: append([]string{}, s.GetJvmOptions()...), CPULimit: float64(s.GetCpuMillis()) / 1000,
			StopTimeout: stopSeconds(s.GetStopTimeoutSeconds()), TimeZone: s.GetTimeZone(),
		},
	}
}

// stopSeconds is the stop timeout of a server in seconds; 0, as older agents send it, means the default.
func stopSeconds(seconds uint32) uint32 {
	return cmp.Or(seconds, uint32(noryxv1.DefaultStopTimeout/time.Second))
}

// listed is a server in a list, with its tags and notes.
type listed struct {
	view
	Tags  []string `json:"tags"`
	Notes string   `json:"notes,omitempty"`
}

// nodeServer is a server with the node it runs on.
type nodeServer struct {
	listed
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
}

// labels are the tags and notes of servers.
type labels struct {
	tags  map[tag.Server][]string
	notes map[tag.Server]string
}

func (h *Handler) labels(ctx context.Context) (l labels, err error) {
	if l.tags, err = h.tags.All(ctx); err == nil {
		l.notes, err = h.tags.Notes(ctx)
	}
	return l, err
}

// visible returns the servers of a node the user may see, with their tags and notes.
func visible(r *http.Request, nodeID string, servers []*noryxv1.Server, l labels) []listed {
	grants := access.From(r.Context())
	views := []listed{}
	for _, s := range servers {
		if grants.On(access.ServersView, nodeID, s.GetId()) {
			ref := tag.Server{NodeID: nodeID, ServerID: s.GetId()}
			views = append(views, listed{toView(s), append([]string{}, l.tags[ref]...), l.notes[ref]})
		}
	}
	return views
}

// listAll returns the servers of all reachable nodes, e.g. to choose the servers of a network.
func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodes.List(r.Context())
	var l labels
	if err == nil {
		l, err = h.labels(r.Context())
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	grants := access.From(r.Context())
	perNode := make([][]nodeServer, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		if n.EnrolledAt == nil || !grants.Somewhere(access.ServersView, n.ID) {
			continue
		}
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
			defer cancel()
			conn, err := h.nodes.Conn(ctx, n.ID)
			if err != nil {
				return
			}
			res, err := noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
			if err != nil {
				return // offline nodes are left out
			}
			for _, s := range visible(r, n.ID, res.GetServers(), l) {
				perNode[i] = append(perNode[i], nodeServer{s, n.ID, n.Name})
			}
		})
	}
	wg.Wait()
	all := []nodeServer{}
	for _, servers := range perNode {
		all = append(all, servers...)
	}
	httpapi.WriteJSON(w, http.StatusOK, all)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	var res *noryxv1.ListServersResponse
	if err == nil {
		res, err = c.ListServers(ctx, &noryxv1.ListServersRequest{})
	}
	var l labels
	if err == nil {
		l, err = h.labels(ctx)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, visible(r, r.PathValue("node"), res.GetServers(), l))
}

// create creates a server as an operation, which downloads the server image if the node
// doesn't have it. Besides the basics, it takes the settings, server.properties and plugins
// a template provides; plugins that can't be installed leave the server without them. A
// modpack decides the type and the versions, and a server without all of it is deleted.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Version    string `json:"version"`
		MemoryMB   uint32 `json:"memoryMb"`
		Port       uint32 `json:"port"`
		AcceptEULA bool   `json:"acceptEula"`
		Storage    string `json:"storage"`
		settings
		Properties map[string]string `json:"properties"`
		// Plugins are the IDs of Modrinth projects to install on the new server.
		Plugins []string `json:"plugins"`
		// Modpack is a version of a Modrinth modpack to install on the new server.
		Modpack *struct {
			Project string `json:"project"`
			Version string `json:"version"`
		} `json:"modpack"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	nodeID := r.PathValue("node")
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	policy, cpuMillis, err := req.check()
	if err == nil && (len(req.Plugins) > 0 || req.Modpack != nil) && !access.From(r.Context()).On(access.Plugins, nodeID, "") {
		err = access.Denied(access.Plugins)
	}
	release := noRelease
	if err == nil {
		release, err = h.checkLimits(ctx, nodeID, "", req.Port, req.MemoryMB)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	steps := []string{"image", "container"}
	if req.Modpack != nil {
		steps = []string{"modpack", "image", "container", "mods"}
	}
	if len(req.Plugins) > 0 {
		steps = append(steps, "plugins")
	}
	spec := operation.Spec{
		Kind: "server.create", Subject: req.Name, NodeID: nodeID, Steps: steps, Status: http.StatusCreated,
		Timeout: createTimeout, Category: logging.Servers, Visible: viewable(nodeID, ""), Cancel: createNeed,
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		defer release()
		create := &noryxv1.CreateServerRequest{
			Name: req.Name, Type: noryxv1.ParseServerType(req.Type), Version: req.Version, MemoryMb: req.MemoryMB,
			Port: req.Port, AcceptEula: req.AcceptEULA, Storage: req.Storage, Java: req.Java, RestartPolicy: policy,
			AikarFlags: req.AikarFlags, JvmOptions: req.JVMOptions, CpuMillis: cpuMillis, Properties: req.Properties,
			LoaderVersion: req.LoaderVersion, StopTimeoutSeconds: req.StopTimeout, TimeZone: req.TimeZone,
		}
		var pack *modpack.Pack
		if req.Modpack != nil {
			operation.Step(ctx, "modpack")
			var err error
			if pack, err = h.modpacks.Resolve(ctx, req.Modpack.Project, req.Modpack.Version); err != nil {
				return nil, err
			}
			create.Type, create.Version, create.LoaderVersion = pack.Type, pack.GameVersion, pack.LoaderVersion
			operation.Step(ctx, "image")
		}
		var res *noryxv1.CreateServerResponse
		err := h.agent(ctx, nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) (err error) {
			res, err = c.CreateServer(ctx, create)
			return err
		})
		if err != nil {
			return nil, err
		}
		id := res.GetServer().GetId()
		operation.Target(ctx, nodeID, id)
		logging.Note(ctx, slog.String(logging.KeyServer, id), slog.String(logging.KeyServerName, res.GetServer().GetName()))
		if pack != nil {
			operation.Step(ctx, "mods")
			err = h.modpacks.Install(ctx, nodeID, id, pack)
		}
		// A server without all of its modpack is deleted, as is one whose creation was
		// cancelled. Once it is complete, it stays: installing its plugins can't be cancelled.
		if err == nil {
			err = operation.Keep(ctx)
		}
		if err != nil {
			return nil, errors.Join(err, h.agent(context.WithoutCancel(ctx), nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) error {
				_, err := c.DeleteServer(ctx, &noryxv1.DeleteServerRequest{Id: id})
				return err
			}))
		}
		created := struct {
			view
			// PluginError tells why the plugins couldn't be installed.
			PluginError string `json:"pluginError,omitempty"`
		}{view: toView(res.GetServer())}
		if len(req.Plugins) > 0 {
			operation.Step(ctx, "plugins")
			if err := h.plugins.InstallOn(ctx, req.Plugins, nodeID, id); err != nil {
				created.PluginError = httpapi.Message(err)
			}
		}
		return created, nil
	})
}

// duplicate copies a server with its data into a new server on the same node, as an
// operation, which takes a while for big worlds.
func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Port uint32 `json:"port"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	nodeID, id := r.PathValue("node"), r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	var list *noryxv1.ListServersResponse
	if err == nil {
		list, err = c.ListServers(ctx, &noryxv1.ListServersRequest{})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	i := slices.IndexFunc(list.GetServers(), func(s *noryxv1.Server) bool { return s.GetId() == id })
	if i < 0 {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusNotFound, "Server not found."))
		return
	}
	source := list.GetServers()[i]
	release, err := h.checkLimits(ctx, nodeID, "", req.Port, source.GetMemoryMb())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	steps := []string{"copy", "container"}
	if source.GetState() != noryxv1.ServerState_SERVER_STATE_STOPPED && !source.GetType().Proxy() {
		steps = append([]string{"save"}, steps...) // it saves its worlds first
	}
	spec := operation.Spec{
		Kind: "server.duplicate", Subject: req.Name, NodeID: nodeID, ServerID: id, Steps: steps, Status: http.StatusCreated,
		Timeout: createTimeout, Category: logging.Servers, Visible: viewable(nodeID, id), Cancel: duplicateNeed,
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		defer release()
		var res *noryxv1.DuplicateServerResponse
		err := h.agent(ctx, nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) (err error) {
			res, err = c.DuplicateServer(ctx, &noryxv1.DuplicateServerRequest{Id: id, Name: req.Name, Port: req.Port})
			return err
		})
		if err != nil {
			return nil, err
		}
		copied := res.GetServer()
		operation.Target(ctx, nodeID, copied.GetId())
		logging.Note(ctx, slog.String("copy", copied.GetName()), slog.String("copy_id", copied.GetId()))
		// The copy exists, also if the operation was cancelled meanwhile.
		ctx = context.WithoutCancel(ctx)
		if err := h.tags.Copy(ctx, tag.Server{NodeID: nodeID, ServerID: id}, tag.Server{NodeID: nodeID, ServerID: copied.GetId()}); err != nil {
			slog.Warn("The copy of a server didn't get its tags and notes", logging.Servers, logging.KeyNode, nodeID, logging.KeyServer, copied.GetId(), "err", err)
		}
		return toView(copied), nil
	})
}

// update changes the settings of a server as an operation: the agent creates its container
// again, after downloading another image for another Java version. Like updateImage, it
// can't be cancelled, which could cut off replacing the container.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		MemoryMB uint32 `json:"memoryMb"`
		Port     uint32 `json:"port"`
		settings
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	nodeID, id := r.PathValue("node"), r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	policy, cpuMillis, err := req.check()
	var current *noryxv1.Server
	if err == nil {
		current, err = h.find(ctx, nodeID, id)
	}
	release := noRelease
	if err == nil {
		release, err = h.checkLimits(ctx, nodeID, id, req.Port, req.MemoryMB)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	spec := operation.Spec{
		Kind: "server.settings", Subject: req.Name, NodeID: nodeID, ServerID: id, Steps: []string{"image", "container"},
		Status: http.StatusOK, Timeout: changeTimeout, Category: logging.Servers, Visible: viewable(nodeID, id),
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		defer release()
		var res *noryxv1.UpdateServerResponse
		err := h.agent(ctx, nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) (err error) {
			res, err = c.UpdateServer(ctx, &noryxv1.UpdateServerRequest{
				Id: id, Name: req.Name, Version: req.Version, MemoryMb: req.MemoryMB, Port: req.Port,
				Java: req.Java, RestartPolicy: policy, AikarFlags: req.AikarFlags, JvmOptions: req.JVMOptions, CpuMillis: cpuMillis,
				LoaderVersion: req.LoaderVersion, StopTimeoutSeconds: req.StopTimeout, TimeZone: req.TimeZone,
			})
			return err
		})
		if err != nil {
			return nil, err
		}
		updated := struct {
			view
			// Warning tells what didn't follow the change, e.g. the proxy of the server's network.
			Warning string `json:"warning,omitempty"`
		}{view: toView(res.GetServer())}
		if res.GetServer().GetPort() != current.GetPort() {
			if err := h.networks.Reapply(ctx, id); err != nil {
				updated.Warning = httpapi.Message(err)
			}
		}
		// Older agents answer without the settings they don't know.
		if updated.StopTimeout != stopSeconds(req.StopTimeout) || updated.TimeZone != req.TimeZone {
			updated.Warning = strings.TrimSpace(updated.Warning + " The agent of the node is too old for a stop timeout and a time zone, so the server keeps 60 seconds and UTC. Update the agent first.")
		}
		return updated, nil
	})
}

// updateImage pulls the server's image again as an operation and, if it changed, recreates
// the container.
func (h *Handler) updateImage(w http.ResponseWriter, r *http.Request) {
	nodeID, id := r.PathValue("node"), r.PathValue("id")
	spec := operation.Spec{
		Kind: "server.image", NodeID: nodeID, ServerID: id, Steps: []string{"image", "container"}, Status: http.StatusOK,
		Timeout: changeTimeout, Category: logging.Servers, Visible: viewable(nodeID, id),
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		var res *noryxv1.UpdateImageResponse
		err := h.agent(ctx, nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) (err error) {
			res, err = c.UpdateImage(ctx, &noryxv1.UpdateImageRequest{Id: id})
			return err
		})
		if err != nil {
			return nil, err
		}
		return map[string]bool{"updated": res.GetUpdated()}, nil
	})
}

// setNotes changes the notes of a server, which doesn't restart it.
func (h *Handler) setNotes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Notes string `json:"notes"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	srv := tag.Server{NodeID: r.PathValue("node"), ServerID: r.PathValue("id")}
	// The notes of a server that doesn't exist would stay.
	_, err := h.find(ctx, srv.NodeID, srv.ServerID)
	if err == nil {
		err = h.tags.SetNotes(ctx, srv, req.Notes)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// delete deletes a server with its data and backups, unless a network needs it.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	nodeID, id := r.PathValue("node"), r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()
	err := h.networks.CheckRemovable(ctx, nodeID, id)
	var c noryxv1.ServerServiceClient
	if err == nil {
		c, err = h.client(ctx, r)
	}
	if err == nil {
		_, err = c.DeleteServer(ctx, &noryxv1.DeleteServerRequest{Id: id})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	// The server is gone: what refers to it is cleaned up as far as possible, even if the
	// request is cancelled.
	ctx, cancel = context.WithTimeout(context.WithoutCancel(r.Context()), queryTimeout)
	defer cancel()
	for _, ref := range h.refs {
		if err := ref.Forget(ctx, nodeID, id); err != nil {
			slog.Warn("Some references to a deleted server were not removed", logging.Servers, logging.KeyNode, nodeID, logging.KeyServer, id, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// startServer starts a server, which only takes a moment, so it answers once the server starts.
func (h *Handler) startServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err == nil {
		err = serverActions["start"].call(ctx, c, r.PathValue("id"), "")
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// power stops or restarts a server as an operation, server.stop or server.restart with the
// step of the same name: a graceful stop may take the server's stop timeout, longer than a
// proxy in front of the master waits for an answer. It can't be cancelled, as a restart cut
// short would leave the server stopped.
func (h *Handler) power(action string) http.HandlerFunc {
	call := serverActions[action].call
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID, id := r.PathValue("node"), r.PathValue("id")
		spec := operation.Spec{
			Kind: "server." + action, NodeID: nodeID, ServerID: id, Steps: []string{action}, Status: http.StatusNoContent,
			Timeout: actionTimeout, Category: logging.Servers, Visible: viewable(nodeID, id),
		}
		h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
			return nil, h.agent(ctx, nodeID, func(ctx context.Context, c noryxv1.ServerServiceClient) error {
				return call(ctx, c, id, "")
			})
		})
	}
}

// agent calls the agent of a node within an operation, which follows the call's progress.
func (h *Handler) agent(ctx context.Context, nodeID string, call func(context.Context, noryxv1.ServerServiceClient) error) error {
	conn, err := h.nodes.Conn(ctx, nodeID)
	if err != nil {
		return err
	}
	ctx, stop := operation.Agent(ctx, conn)
	defer stop()
	return call(ctx, noryxv1.NewServerServiceClient(conn))
}

// viewable lets those see an operation who may see its server, or the whole node.
func viewable(nodeID, serverID string) func(access.Grants) bool {
	return func(g access.Grants) bool { return g.On(access.ServersView, nodeID, serverID) }
}

// client returns a client of the agent of the node in the path.
func (h *Handler) client(ctx context.Context, r *http.Request) (noryxv1.ServerServiceClient, error) {
	return h.serverClient(ctx, r.PathValue("node"))
}

func (h *Handler) serverClient(ctx context.Context, nodeID string) (noryxv1.ServerServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return noryxv1.NewServerServiceClient(conn), nil
}
