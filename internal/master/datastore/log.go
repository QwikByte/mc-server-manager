package datastore

import (
	"context"
	"net/http"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// Log streams the log of a datastore's container, see httpapi.StreamLog, in which its agent hides
// passwords. Its stream ends when the container stops.
func (s *Service) Log(w http.ResponseWriter, r *http.Request, id string) {
	ds, err := s.store.get(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.StreamLog(w, r, func(ctx context.Context, tail uint32, after int64) (httpapi.Stream[*noryxv1.StreamDatastoreLogsResponse], error) {
		conn, err := s.nodes.Conn(ctx, ds.NodeID)
		if err != nil {
			return nil, err
		}
		return noryxv1.NewDatastoreServiceClient(conn).StreamDatastoreLogs(ctx, &noryxv1.StreamDatastoreLogsRequest{Id: id, Tail: tail, AfterUnixNano: after})
	})
}
