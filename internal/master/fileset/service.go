package fileset

import (
	"context"
	"crypto/rand"
	"database/sql"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// reconcileEvery is how often the servers lose the files with secrets of sets that are no
// longer for them, in case a node couldn't be reached when that changed.
const reconcileEvery = 15 * time.Minute

type Service struct {
	store      store
	nodes      Nodes
	networks   Networks
	tags       Tags
	moves      Moves
	datastores Datastores
}

func NewService(db *sql.DB, nodes Nodes, networks Networks, tags Tags, moves Moves, datastores Datastores) *Service {
	return &Service{store: store{db}, nodes: nodes, networks: networks, tags: tags, moves: moves, datastores: datastores}
}

func (s *Service) List(ctx context.Context) ([]Summary, error) { return s.store.summaries(ctx) }

func (s *Service) Get(ctx context.Context, id string) (Set, error) { return s.store.get(ctx, id) }

// Version returns a kept version of a set with its files.
func (s *Service) Version(ctx context.Context, id string, version int64) (Version, error) {
	if _, err := s.store.get(ctx, id); err != nil {
		return Version{}, err
	}
	return s.store.version(ctx, id, version)
}

func (s *Service) Create(ctx context.Context, in Input, user string) (Set, error) {
	if err := s.check(ctx, &in); err != nil {
		return Set{}, err
	}
	id := strings.ToLower(rand.Text())
	if _, err := s.store.save(ctx, id, in, user, true); err != nil {
		return Set{}, err
	}
	return s.store.get(ctx, id)
}

// Update saves a change of a set. Servers that it is no longer for lose its files with
// secrets right away, in the background; changing the files changes no server until the set
// is applied.
func (s *Service) Update(ctx context.Context, id string, in Input, user string) (Set, error) {
	if err := s.check(ctx, &in); err != nil {
		return Set{}, err
	}
	if _, err := s.store.save(ctx, id, in, user, false); err != nil {
		return Set{}, err
	}
	go s.reconcile(ctx, nil)
	return s.store.get(ctx, id)
}

// Delete deletes a set. Its files with secrets are removed from the servers in the background;
// the others stay.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.store.delete(ctx, id); err != nil {
		return err
	}
	go s.reconcile(ctx, nil)
	return nil
}

func (s *Service) check(ctx context.Context, in *Input) error {
	networks, err := s.networks.List(ctx)
	if err != nil {
		return err
	}
	return in.check(func(id string) bool {
		return slices.ContainsFunc(networks, func(n network.Network) bool { return n.ID == id })
	})
}

// SetSecret sets the value of a secret of a set, or a new random one if generate is set.
func (s *Service) SetSecret(ctx context.Context, id, name, value string, generate bool) (Secret, error) {
	if generate {
		value = rand.Text()
	}
	if !noryxv1.SecretName.MatchString(name) {
		return Secret{}, httpapi.Errorf(http.StatusBadRequest, "Secret names have up to 64 lower-case letters, digits, - and _.")
	}
	if problem := noryxv1.SecretValueProblem(value); problem != "" {
		return Secret{}, httpapi.Errorf(http.StatusBadRequest, "%s", problem)
	}
	if _, err := s.store.get(ctx, id); err != nil {
		return Secret{}, err
	}
	return s.store.setSecret(ctx, id, name, value, noryxv1.MaxFileSetSecrets)
}

func (s *Service) DeleteSecret(ctx context.Context, id, name string) error {
	return s.store.deleteSecret(ctx, id, name)
}

// Status returns the state of a set on the servers it is for, or that still have its files.
func (s *Service) Status(ctx context.Context, id string) ([]ServerStatus, error) {
	set, err := s.store.get(ctx, id)
	if err != nil {
		return nil, err
	}
	values, err := s.store.secrets(ctx, id)
	if err != nil {
		return nil, err
	}
	sv, err := s.survey(ctx)
	if err != nil {
		return nil, err
	}
	return sv.status(set, values), nil
}

// StatusAll returns the states of all sets, by set, with one survey of the nodes.
func (s *Service) StatusAll(ctx context.Context) (map[string][]ServerStatus, error) {
	list, err := s.store.summaries(ctx)
	if err != nil {
		return nil, err
	}
	sv, err := s.survey(ctx)
	if err != nil {
		return nil, err
	}
	all := map[string][]ServerStatus{}
	for _, sum := range list {
		set, err := s.store.get(ctx, sum.ID)
		var values map[string]string
		if err == nil {
			values, err = s.store.secrets(ctx, sum.ID)
		}
		if err != nil {
			return nil, err
		}
		all[sum.ID] = sv.status(set, values)
	}
	return all, nil
}

// Left takes the files with secrets of sets off servers that the sets are no longer for,
// e.g. as they lost a tag or left a network. Servers of nodes that can't be reached lose
// them later.
func (s *Service) Left(ctx context.Context, servers []tag.Server) {
	s.reconcile(ctx, func(ref tag.Server) bool { return slices.Contains(servers, ref) })
}

// Run takes the files with secrets of sets off the servers they are no longer for every
// reconcileEvery, until ctx is done.
func (s *Service) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconcileEvery):
			s.reconcile(ctx, nil)
		}
	}
}

// reconcile takes the files with secrets of sets off the servers, among those only allows
// (all if nil), that the sets are no longer for, and forgets the sets that were deleted,
// whose other files stay. Moving servers are left alone.
func (s *Service) reconcile(ctx context.Context, only func(tag.Server) bool) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	sets, err := s.store.targets(ctx)
	var ids []string
	if err == nil {
		ids, err = s.store.ids(ctx)
	}
	var sv *survey
	if err == nil {
		sv, err = s.survey(ctx)
	}
	if err != nil {
		slog.Warn("Can't tell which servers file sets are no longer for", logging.Files, "err", err)
		return
	}
	targets := map[string][]tag.Server{}
	for _, id := range ids {
		targets[id] = sv.targets(sets[id])
	}
	for ref, applied := range sv.applied {
		if only != nil && !only(ref) || s.moves.Check(ref.ServerID) != nil {
			continue
		}
		for _, a := range applied {
			removal := noryxv1.FileSetRemoval_FILE_SET_REMOVAL_SECRETS
			switch _, exists := targets[a.GetSetId()]; {
			case !exists:
				removal = noryxv1.FileSetRemoval_FILE_SET_REMOVAL_FORGET
			case slices.Contains(targets[a.GetSetId()], ref) || !slices.ContainsFunc(a.GetFiles(), (*noryxv1.AppliedFile).GetSecret):
				continue
			}
			if _, err := s.remove(ctx, ref, a.GetSetId(), removal, false); err != nil {
				slog.Warn("Can't take the files with secrets of a file set off a server", logging.Files,
					logging.KeyNode, ref.NodeID, logging.KeyServer, ref.ServerID, "set", a.GetSetId(), "err", err)
			}
		}
	}
}

// remove takes a set off a server.
func (s *Service) remove(ctx context.Context, ref tag.Server, setID string, removal noryxv1.FileSetRemoval, dry bool) ([]*noryxv1.FileSetChange, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return nil, err
	}
	res, err := noryxv1.NewFileSetServiceClient(conn).RemoveFileSet(ctx, &noryxv1.RemoveFileSetRequest{
		ServerId: ref.ServerID, SetId: setID, Removal: removal, DryRun: dry,
	})
	return res.GetChanges(), err
}
