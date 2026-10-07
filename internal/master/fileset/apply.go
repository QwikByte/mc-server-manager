package fileset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

const (
	// applyTimeout covers writing the files of a server, also when it is large.
	applyTimeout = 2 * time.Minute
	// restartTimeout covers a graceful stop, which may take the longest stop timeout, and a start.
	restartTimeout = 2*time.Minute + noryxv1.MaxStopTimeout
	// maxBatch is the most game servers of a network that restart at a time, as for rolling restarts.
	maxBatch = 50
)

// Change is what applying a set does with a file of a server.
type Change struct {
	Path   string `json:"path"`
	Action string `json:"action"` // unchanged, created, changed, removed or kept
	Secret bool   `json:"secret,omitempty"`
}

// PreviewFile is what applying a set does with a file of a server, and the files before and
// after, as keys of the contents of the preview. Files with secrets show the version the
// server has and the new one, with placeholders instead of the secrets.
type PreviewFile struct {
	Change
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	// ChangedOnServer tells that a file with secrets changed on the server since it was written.
	ChangedOnServer bool `json:"changedOnServer,omitempty"`
	// Unknown tells that the file before can't be shown, e.g. as it is too large.
	Unknown bool `json:"unknown,omitempty"`
}

// PreviewServer is what applying a set does on a server.
type PreviewServer struct {
	ServerStatus
	// FirstSecrets tells that the server gets secrets of the set for the first time.
	FirstSecrets bool          `json:"firstSecrets,omitempty"`
	Files        []PreviewFile `json:"files"`
	// Error tells why the set can't be applied to the server.
	Error string `json:"error,omitempty"`
}

// Preview is what applying a version of a set does on each server.
type Preview struct {
	Version int64           `json:"version"`
	Servers []PreviewServer `json:"servers"`
	// Contents are the texts of the files, by their SHA-256, so that servers with the same
	// files share them.
	Contents map[string]string `json:"contents"`
}

// Result tells how applying a set ended on a server.
type Result struct {
	ServerStatus
	Changes []Change `json:"changes"`
	Error   string   `json:"error,omitempty"`
	// Restart tells that the server runs with files it didn't load yet, Restarted that it
	// was restarted to load them.
	Restart   bool `json:"restart,omitempty"`
	Restarted bool `json:"restarted,omitempty"`
}

// ApplyRequest applies the version of a set that was previewed.
type ApplyRequest struct {
	Version int64 `json:"version"`
	// Restart restarts the running servers whose files changed, the game servers of a
	// network Batch at a time.
	Restart bool `json:"restart"`
	Batch   int  `json:"batch"`
	// Servers, if set, limit applying to these of its servers, e.g. to those where it failed.
	Servers []tag.Server `json:"servers,omitempty"`
}

// target is a server that applying a set touches, as the survey found it.
type target struct {
	ServerStatus
	member   member
	rendered rendered
	applied  *noryxv1.AppliedFileSet
}

// targets returns the servers that applying a set touches, after allowed checked each.
func (s *Service) targets(ctx context.Context, id string, version int64, allowed func(tag.Server) error) (Set, *survey, []target, error) {
	set, err := s.store.get(ctx, id)
	if err != nil {
		return set, nil, nil, err
	}
	if version != set.Version {
		return set, nil, nil, httpapi.Errorf(http.StatusConflict, "The file set changed since you opened it. Open it again to see the changes.")
	}
	values, err := s.store.secrets(ctx, id)
	if err != nil {
		return set, nil, nil, err
	}
	sv, err := s.survey(ctx)
	if err != nil {
		return set, nil, nil, err
	}
	var list []target
	for _, st := range sv.status(set, values) {
		ref := tag.Server{NodeID: st.NodeID, ServerID: st.ServerID}
		if err := allowed(ref); err != nil {
			return set, nil, nil, err
		}
		t := target{ServerStatus: st, member: sv.member(ref), applied: sv.set(ref, id)}
		t.rendered, _ = render(id, set.Files, values, t.member) // a problem is in the status
		list = append(list, t)
	}
	if len(list) == 0 {
		return set, nil, nil, httpapi.Errorf(http.StatusConflict, "The file set is for no server yet. Add a tag or network as target.")
	}
	return set, sv, list, nil
}

// problem returns why a set can't be applied to a server, or nil.
func (s *Service) problem(t target) error {
	switch {
	case t.State == Unreachable || t.State != Left && t.Problem != "":
		return httpapi.Errorf(http.StatusConflict, "%s", t.Problem)
	}
	return s.moves.Check(t.ServerID)
}

// call applies a set to a server, or takes it off one the set is no longer for.
func (s *Service) call(ctx context.Context, set Set, t target, dry bool) ([]*noryxv1.FileSetChange, error) {
	if t.State == Left {
		return s.remove(ctx, t.member.Server, set.ID, noryxv1.FileSetRemoval_FILE_SET_REMOVAL_UNCHANGED, dry)
	}
	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, t.NodeID)
	if err != nil {
		return nil, err
	}
	res, err := noryxv1.NewFileSetServiceClient(conn).ApplyFileSet(ctx, &noryxv1.ApplyFileSetRequest{
		ServerId: t.ServerID, SetId: set.ID, SetName: set.Name, Version: set.Version, Revision: t.rendered.revision,
		Files: t.rendered.files, Secrets: t.rendered.secrets, DryRun: dry,
	})
	return res.GetChanges(), err
}

// Preview tells what applying a version of a set does on each server, after allowed checked
// each. It shows no secrets.
func (s *Service) Preview(ctx context.Context, id string, version int64, allowed func(tag.Server) error) (Preview, error) {
	set, _, list, err := s.targets(ctx, id, version, allowed)
	if err != nil {
		return Preview{}, err
	}
	p := Preview{Version: set.Version, Servers: make([]PreviewServer, len(list)), Contents: map[string]string{}}
	var mu sync.Mutex
	put := func(content string) string {
		sum := sha256.Sum256([]byte(content))
		key := hex.EncodeToString(sum[:])
		mu.Lock()
		defer mu.Unlock()
		p.Contents[key] = content
		return key
	}
	operation.Each(ctx, nodesOf(list), func(ctx context.Context, i int) {
		t := list[i]
		p.Servers[i] = PreviewServer{ServerStatus: t.ServerStatus, Files: []PreviewFile{}}
		err := s.problem(t)
		var changes []*noryxv1.FileSetChange
		if err == nil {
			changes, err = s.call(ctx, set, t, true)
		}
		if err != nil {
			p.Servers[i].Error = httpapi.Message(err)
			return
		}
		had := slices.ContainsFunc(t.applied.GetFiles(), (*noryxv1.AppliedFile).GetSecret)
		for _, c := range changes {
			f, action := PreviewFile{Change: change(c)}, c.GetAction()
			written := action == noryxv1.FileSetAction_FILE_SET_ACTION_CREATED || action == noryxv1.FileSetAction_FILE_SET_ACTION_CHANGED
			if after, ok := t.rendered.shown(c.GetPath()); ok && written {
				f.After = put(after)
			}
			if action != noryxv1.FileSetAction_FILE_SET_ACTION_UNCHANGED && action != noryxv1.FileSetAction_FILE_SET_ACTION_CREATED {
				var before string
				var err error
				if c.GetSecret() {
					f.ChangedOnServer = slices.ContainsFunc(t.applied.GetFiles(), func(a *noryxv1.AppliedFile) bool { return a.GetPath() == c.GetPath() && a.GetChanged() })
					before, err = s.applied(ctx, set, t, c.GetPath())
				} else {
					before, err = s.read(ctx, t.member.Server, c.GetPath())
				}
				if f.Unknown = err != nil; !f.Unknown {
					f.Before = put(before)
				}
			}
			p.Servers[i].FirstSecrets = p.Servers[i].FirstSecrets || c.GetSecret() && written && !had
			p.Servers[i].Files = append(p.Servers[i].Files, f)
		}
	}, nil)
	return p, nil
}

// applied returns a file with secrets of a set as the server has it, with placeholders
// instead of the secrets: from the version it has, as far as that is kept.
func (s *Service) applied(ctx context.Context, set Set, t target, path string) (string, error) {
	v, err := s.store.version(ctx, set.ID, t.applied.GetVersion())
	if err != nil {
		return "", err
	}
	r, _ := render(set.ID, v.Files, nil, t.member)
	content, ok := r.shown(path)
	if !ok {
		return "", errors.New("not in the version")
	}
	return content, nil
}

// read reads a file of a server, as the file manager shows it, up to the size of a file of a set.
func (s *Service) read(ctx context.Context, ref tag.Server, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return "", err
	}
	stream, err := noryxv1.NewFileServiceClient(conn).ReadFile(ctx, &noryxv1.ReadFileRequest{ServerId: ref.ServerID, Path: path})
	if err != nil {
		return "", err
	}
	var data []byte
	for {
		res, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if data = append(data, res.GetData()...); len(data) > noryxv1.MaxFileSetFileSize {
			return "", errors.New("too large")
		}
	}
	if problem := noryxv1.FileSetContentProblem(path, string(data)); problem != "" {
		return "", errors.New(problem)
	}
	return string(data), nil
}

// Apply applies the version of a set that was previewed to the servers it is for, and takes
// it off those it is no longer for, after allowed checked each. Moving servers and those of
// nodes that can't be reached are left out. It can restart the running servers whose files
// changed: the game servers of a network a few at a time, so that it stays open. Once it
// restarts servers, it can't be cancelled.
func (s *Service) Apply(ctx context.Context, id string, req ApplyRequest, allowed func(tag.Server) error) ([]Result, error) {
	switch {
	case req.Restart && req.Batch == 0:
		req.Batch = 1
	case req.Restart && (req.Batch < 1 || req.Batch > maxBatch):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Restart from 1 to %d servers of a network at a time.", maxBatch)
	}
	set, sv, list, err := s.targets(ctx, id, req.Version, allowed)
	if err != nil {
		return nil, err
	}
	if len(req.Servers) > 0 {
		chosen := map[tag.Server]bool{}
		for _, ref := range req.Servers {
			chosen[ref] = true
		}
		list = slices.DeleteFunc(list, func(t target) bool { return !chosen[tag.Server{NodeID: t.NodeID, ServerID: t.ServerID}] })
		if len(list) == 0 {
			return nil, httpapi.Errorf(http.StatusConflict, "The file set is no longer for these servers.")
		}
	}
	operation.Step(ctx, "files")
	results := make([]Result, len(list))
	for i, t := range list {
		results[i] = Result{ServerStatus: t.ServerStatus, Changes: []Change{}}
	}
	failed := func(i int, err error) { results[i].Error = httpapi.Message(err) }
	operation.Each(ctx, nodesOf(list), func(ctx context.Context, i int) {
		t := list[i]
		err := s.problem(t)
		var changes []*noryxv1.FileSetChange
		if err == nil {
			changes, err = s.call(ctx, set, t, false)
		}
		if err != nil {
			failed(i, err)
		}
		for _, c := range changes {
			results[i].Changes = append(results[i].Changes, change(c))
			// Kept files stay as they were, and the agent tells removed files that were gone already as unchanged.
			results[i].Restart = results[i].Restart || t.Running && slices.Contains([]noryxv1.FileSetAction{
				noryxv1.FileSetAction_FILE_SET_ACTION_CREATED, noryxv1.FileSetAction_FILE_SET_ACTION_CHANGED, noryxv1.FileSetAction_FILE_SET_ACTION_REMOVED,
			}, c.GetAction())
		}
	}, failed)
	if !req.Restart {
		return results, nil
	}
	if err := operation.Keep(ctx); err != nil {
		return results, err // nothing restarts; the results tell which servers need it
	}
	operation.Step(ctx, "restart")
	s.restart(ctx, sv, results, req.Batch)
	return results, nil
}

// restart restarts the servers that need it to load their files: the game servers of each
// network a batch at a time, then the others. Restarts can't be cancelled, and each takes the
// time it needs, also once rolling restarts of large networks outlasted the deadline of ctx.
func (s *Service) restart(ctx context.Context, sv *survey, results []Result, batch int) {
	ctx = context.WithoutCancel(ctx)
	pending := map[tag.Server]*Result{}
	for i, r := range results {
		if r.Restart {
			pending[tag.Server{NodeID: r.NodeID, ServerID: r.ServerID}] = &results[i]
		}
	}
	done := func(refs []tag.Server, err error) {
		for _, ref := range refs {
			r := pending[ref]
			r.Restart, r.Restarted = err != nil, err == nil
			if err != nil {
				r.Error = fmt.Sprintf("It couldn't restart: %s", httpapi.Message(err))
			}
			delete(pending, ref)
		}
	}
	for _, n := range sv.networks {
		var refs []tag.Server
		var only []network.Ref
		for _, b := range n.Backends {
			if pending[tag.Server(b.Ref)] != nil {
				refs, only = append(refs, tag.Server(b.Ref)), append(only, b.Ref)
			}
		}
		if len(only) > 0 {
			done(refs, s.networks.RollingRestart(ctx, n, batch, only...))
		}
	}
	var rest []tag.Server
	for ref := range pending {
		rest = append(rest, ref)
	}
	nodes := make([]string, len(rest))
	for i, ref := range rest {
		nodes[i] = ref.NodeID
	}
	errs := make([]error, len(rest))
	operation.Each(ctx, nodes, func(ctx context.Context, i int) {
		ctx, cancel := context.WithTimeout(ctx, restartTimeout)
		defer cancel()
		conn, err := s.nodes.Conn(ctx, rest[i].NodeID)
		if err == nil {
			_, err = noryxv1.NewServerServiceClient(conn).RestartServer(ctx, &noryxv1.RestartServerRequest{Id: rest[i].ServerID})
		}
		errs[i] = err
	}, nil)
	for i, ref := range rest {
		done([]tag.Server{ref}, errs[i])
	}
}

func change(c *noryxv1.FileSetChange) Change {
	return Change{Path: c.GetPath(), Action: c.GetAction().Slug(), Secret: c.GetSecret()}
}

func nodesOf(list []target) []string {
	nodes := make([]string, len(list))
	for i, t := range list {
		nodes[i] = t.NodeID
	}
	return nodes
}
