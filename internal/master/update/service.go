// Package update tells administrators about new releases of MC Server Manager and installs
// them. The master may not install anything itself: it asks a systemd unit that runs as root
// to install the latest release, restarts on it and then updates the agents through their
// connections.
package update

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/mod/semver"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/logging"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
)

const (
	// DefaultAPI describes the latest release; GitHub leaves pre-releases out.
	DefaultAPI = "https://api.github.com/repos/QwikByte/mc-server-manager/releases/latest"
	// DefaultUnit makes systemd install the latest release when the master asks for it.
	DefaultUnit = "/usr/lib/systemd/system/mcsm-master-update.path"

	checkEvery   = 6 * time.Hour
	probeTimeout = 5 * time.Second
	maxResponse  = 1 << 20
	// pending is how long an update may take before the panel reports it as failed.
	pending = 5 * time.Minute
)

// Release is a published release of MC Server Manager.
type Release struct {
	Version     string    `json:"version"`
	Notes       string    `json:"notes"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
}

// Status is what administrators see about updates.
type Status struct {
	Version    string     `json:"version"`          // of the master
	Latest     *Release   `json:"latest,omitempty"` // the latest release, if newer than the master
	CheckedAt  *time.Time `json:"checkedAt,omitempty"`
	CheckError string     `json:"checkError,omitempty"`
	// Updatable tells whether the master can update itself; otherwise it is updated on its host.
	Updatable bool      `json:"updatable"`
	Master    *Progress `json:"master,omitempty"`
	// Agents are the online agents that are older than the master.
	Agents []Agent `json:"agents"`
}

// Progress is an update that was asked for and hasn't finished yet, or failed.
type Progress struct {
	Since time.Time `json:"since"`
	Error string    `json:"error,omitempty"`
}

// Agent is an agent that is older than the master.
type Agent struct {
	NodeID  string    `json:"nodeId"`
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Update  *Progress `json:"update,omitempty"`
}

// Config tells whether to look for new releases.
type Config interface{ CheckUpdates() bool }

// Options locate what the service uses.
type Options struct {
	// DataDir is the master's; mcsm-master-update.path watches the file update-request in it.
	DataDir string
	API     string // DefaultAPI if empty
	Unit    string // DefaultUnit if empty
}

type Service struct {
	nodes  *node.Service
	config Config
	client *http.Client
	api    string
	unit   string
	// request makes systemd update the master, followUp makes the master update the
	// agents once it runs the new release.
	request, followUp string

	mu       sync.Mutex
	latest   *Release
	checked  time.Time
	checkErr string
	master   time.Time           // when the update of the master was asked for
	agents   map[string]Progress // by node ID
}

func New(nodes *node.Service, config Config, o Options) *Service {
	return &Service{
		nodes: nodes, config: config, client: &http.Client{Timeout: 30 * time.Second},
		api: cmp.Or(o.API, DefaultAPI), unit: cmp.Or(o.Unit, DefaultUnit),
		request: filepath.Join(o.DataDir, "update-request"), followUp: filepath.Join(o.DataDir, "update-agents"),
		agents: map[string]Progress{},
	}
}

// Run updates the agents if the master was just updated from the panel, then looks for new
// releases every few hours while the settings allow. Builds that aren't a release don't.
func (s *Service) Run(ctx context.Context) {
	if os.Remove(s.followUp) == nil {
		_ = s.UpdateAgents(ctx) // logged for each agent
	}
	if !buildinfo.IsRelease(buildinfo.Version) {
		return
	}
	for {
		if s.config.CheckUpdates() {
			s.Check(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(checkEvery):
		}
	}
}

// Check looks for the latest release now. A failure shows in the status.
func (s *Service) Check(ctx context.Context) {
	latest, err := s.fetch(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checked = time.Now()
	if err != nil {
		s.checkErr = err.Error()
		slog.Warn("Check for updates failed", logging.System, "err", err)
		return
	}
	known := s.latest
	s.latest, s.checkErr = latest, ""
	if s.available() && (known == nil || known.Version != latest.Version) {
		slog.Info("Update available", logging.System, "version", latest.Version)
	}
}

// fetch returns the latest release, or nil if there is none yet.
func (s *Service) fetch(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var r struct {
		Tag       string    `json:"tag_name"`
		Body      string    `json:"body"`
		Published time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponse)).Decode(&r); err != nil {
		return nil, fmt.Errorf("read the latest release: %w", err)
	}
	if !buildinfo.IsRelease(r.Tag) {
		return nil, fmt.Errorf("the latest release has the unexpected version %q", r.Tag)
	}
	return &Release{r.Tag, r.Body, buildinfo.Repository + "/releases/tag/" + r.Tag, r.Published}, nil
}

// available tells whether the latest release is newer than the master. Callers hold mu.
func (s *Service) available() bool {
	return s.latest != nil && semver.Compare(s.latest.Version, buildinfo.Version) > 0
}

// updatable tells whether systemd installs releases when the master asks for it.
func (s *Service) updatable() bool {
	_, err := os.Stat(s.unit)
	return err == nil
}

// Status returns what administrators see about updates.
func (s *Service) Status(ctx context.Context) (Status, error) {
	agents, err := s.outdated(ctx)
	if err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{Version: buildinfo.Version, CheckError: s.checkErr, Updatable: s.updatable(), Agents: agents}
	if s.available() {
		st.Latest = s.latest
	}
	if checked := s.checked; !checked.IsZero() {
		st.CheckedAt = &checked
	}
	if !s.master.IsZero() {
		st.Master = &Progress{Since: s.master}
		if time.Since(s.master) > pending {
			st.Master.Error = "The update didn't finish. See: journalctl -u mcsm-master-update"
			if _, err := os.Stat(s.request); err == nil {
				st.Master.Error = "The update didn't start. See: systemctl status mcsm-master-update.path"
			}
		}
	}
	outdated := map[string]bool{}
	for i, a := range st.Agents {
		outdated[a.NodeID] = true
		if p, ok := s.agents[a.NodeID]; ok {
			if p.Error == "" && time.Since(p.Since) > pending {
				p.Error = "The update didn't finish. See on the node: journalctl -u mcsm-agent-update"
			}
			st.Agents[i].Update = &p
		}
	}
	for id := range s.agents {
		if !outdated[id] { // updated, or offline
			delete(s.agents, id)
		}
	}
	return st, nil
}

// UpdateMaster asks systemd to install the latest release. The master restarts on it and
// then updates the agents.
func (s *Service) UpdateMaster(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case !s.updatable():
		return httpapi.Errorf(http.StatusConflict, "This master wasn't installed from a package. Update it on its host with: "+
			"curl -fsSL %s/releases/latest/download/install.sh | sudo bash -s -- update", buildinfo.Repository)
	case !s.available():
		return httpapi.Errorf(http.StatusConflict, "There is no newer release.")
	}
	// The follow-up comes first: the master may restart as soon as the request exists.
	if err := os.WriteFile(s.followUp, nil, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(s.request, nil, 0o600); err != nil {
		return err
	}
	s.master = time.Now()
	logging.Note(ctx, slog.String("version", s.latest.Version))
	return nil
}

// UpdateAgents updates the online agents that are older than the master to its version,
// except those already updating.
func (s *Service) UpdateAgents(ctx context.Context) error {
	agents, err := s.outdated(ctx)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, a := range agents {
		s.mu.Lock()
		p, ok := s.agents[a.NodeID]
		s.mu.Unlock()
		if !ok || p.Error != "" || time.Since(p.Since) > pending {
			wg.Go(func() { s.updateAgent(ctx, a) })
		}
	}
	wg.Wait()
	return nil
}

func (s *Service) updateAgent(ctx context.Context, a Agent) {
	p := Progress{Since: time.Now()}
	conn, err := s.nodes.Conn(ctx, a.NodeID)
	if err == nil {
		_, err = mcsmv1.NewNodeServiceClient(conn).Update(ctx, &mcsmv1.UpdateRequest{Version: buildinfo.Version})
	}
	attrs := []any{logging.Nodes, logging.KeyNode, a.NodeID, logging.KeyNodeName, a.Name, "version", buildinfo.Version}
	if err != nil {
		p.Error = status.Convert(err).Message()
		slog.Warn("Update agent failed", append(attrs, "err", err)...)
	} else {
		slog.Info("Update agent", attrs...)
	}
	s.mu.Lock()
	s.agents[a.NodeID] = p
	s.mu.Unlock()
}

// outdated returns the online agents that are older than the master.
func (s *Service) outdated(ctx context.Context) ([]Agent, error) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	found := make([]*Agent, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		if n.EnrolledAt == nil {
			continue
		}
		wg.Go(func() {
			info, _, err := s.nodes.Status(ctx, n.ID)
			if err == nil && semver.Compare(info.GetAgentVersion(), buildinfo.Version) < 0 {
				found[i] = &Agent{NodeID: n.ID, Name: n.Name, Version: info.GetAgentVersion()}
			}
		})
	}
	wg.Wait()
	agents := []Agent{}
	for _, a := range found {
		if a != nil {
			agents = append(agents, *a)
		}
	}
	return agents, nil
}
