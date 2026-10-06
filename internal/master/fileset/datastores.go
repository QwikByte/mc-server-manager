package fileset

import (
	"context"
	"slices"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// Use is a server that a set with placeholders of a database is for.
type Use struct {
	ServerStatus
	SetID   string `json:"setId"`
	SetName string `json:"setName"`
	// Database is the database whose fields the set has, as <datastore>.<database>.
	Database string `json:"database"`
}

// Uses returns the servers of a network that sets with placeholders of databases are for.
func (s *Service) Uses(ctx context.Context, networkID string) ([]Use, error) {
	list, err := s.store.summaries(ctx)
	if err != nil {
		return nil, err
	}
	sv, err := s.survey(ctx)
	if err != nil {
		return nil, err
	}
	uses := []Use{}
	for _, sum := range list {
		set, err := s.store.get(ctx, sum.ID)
		var values map[string]string
		if err == nil {
			values, err = s.store.secrets(ctx, sum.ID)
		}
		if err != nil {
			return nil, err
		}
		databases := databasesOf(set.Files)
		if len(databases) == 0 {
			continue
		}
		for _, st := range sv.status(set, values) {
			if st.State == Left || sv.member(tag.Server{NodeID: st.NodeID, ServerID: st.ServerID}).NetworkID != networkID {
				continue
			}
			for _, db := range databases {
				uses = append(uses, Use{ServerStatus: st, SetID: set.ID, SetName: set.Name, Database: db})
			}
		}
	}
	return uses, nil
}

// databasesOf returns the databases whose fields files have, as <datastore>.<database>.
func databasesOf(files []File) []string {
	var databases []string
	for _, f := range files {
		for _, m := range noryxv1.Placeholder.FindAllStringSubmatch(f.Content, -1) {
			if parts := noryxv1.DatastoreField.FindStringSubmatch(m[1]); parts != nil {
				databases = append(databases, parts[1]+"."+parts[2])
			}
		}
	}
	slices.Sort(databases)
	return slices.Compact(databases)
}

// Reapply applies the current version of a set again, e.g. as the password of a database it
// uses changed, and restarts the running servers whose files changed.
func (s *Service) Reapply(ctx context.Context, id string) ([]Result, error) {
	set, err := s.store.get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.Apply(ctx, id, ApplyRequest{Version: set.Version, Restart: true}, func(tag.Server) error { return nil })
}
