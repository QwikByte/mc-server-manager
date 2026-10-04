// Package logs keeps the latest entries of the agent's log in memory, serves them to the
// master and the local CLI, and logs every call the agent receives.
package logs

import (
	"crypto/rand"
	"log/slog"
	"slices"
	"sort"
	"sync"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/logging"
)

const (
	keep      = 5000 // entries kept in memory
	chunkSize = 500  // entries per message
)

// Buffer keeps the latest entries of the log and tells readers about new ones.
type Buffer struct {
	boot string

	mu      sync.Mutex
	entries []*noryxv1.LogEntry // oldest first, never changed once added
	seq     uint64
	changed chan struct{} // closed when an entry is added
}

func NewBuffer() *Buffer { return &Buffer{boot: rand.Text(), changed: make(chan struct{})} }

// Handler returns a slog handler that adds the records of at least level to the buffer.
func (b *Buffer) Handler(level slog.Leveler) slog.Handler {
	return logging.NewHandler(level, func(e logging.Entry) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.entries) == keep {
			b.entries = slices.Delete(b.entries, 0, keep/10) // drops in batches, not on every entry
		}
		b.seq++
		b.entries = append(b.entries, &noryxv1.LogEntry{
			Boot: b.boot, Seq: b.seq, TimeUnixNano: e.Time.UnixNano(), Level: int32(e.Level), Message: e.Message, //nolint:gosec // levels are normalized to -4 to 8
			Category: e.Category, ServerId: e.Server, Attrs: e.Attrs,
		})
		close(b.changed)
		b.changed = make(chan struct{})
	})
}

// after returns the entries after a position, and a channel that is closed once there are newer ones.
func (b *Buffer) after(boot string, seq uint64) ([]*noryxv1.LogEntry, <-chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if boot != b.boot {
		seq = 0
	}
	i := sort.Search(len(b.entries), func(i int) bool { return b.entries[i].GetSeq() > seq })
	return slices.Clone(b.entries[i:]), b.changed
}

// Service implements noryxv1.LogServiceServer.
type Service struct {
	noryxv1.UnimplementedLogServiceServer
	buf *Buffer
}

func NewService(buf *Buffer) *Service { return &Service{buf: buf} }

func (s *Service) ReadLog(req *noryxv1.ReadLogRequest, stream grpc.ServerStreamingServer[noryxv1.ReadLogResponse]) error {
	boot, seq := req.GetBoot(), req.GetAfter()
	for {
		entries, changed := s.buf.after(boot, seq)
		for chunk := range slices.Chunk(entries, chunkSize) {
			if err := stream.Send(&noryxv1.ReadLogResponse{Entries: chunk}); err != nil {
				return err
			}
		}
		if len(entries) > 0 {
			boot, seq = s.buf.boot, entries[len(entries)-1].GetSeq()
		}
		if !req.GetFollow() {
			return nil
		}
		select {
		case <-stream.Context().Done():
			return nil
		case <-changed:
		}
	}
}
