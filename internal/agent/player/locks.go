package player

import (
	"context"
	"sync"

	"google.golang.org/grpc/status"
)

// serverLocks lock servers one by one. A server's lock exists only while it is held or waited
// for, so that deleted servers leave none behind.
type serverLocks struct {
	mu   sync.Mutex
	byID map[string]*serverLock
}

type serverLock struct {
	held  chan struct{} // has a value while the lock is held
	users int           // that hold the lock or wait for it
}

// lock locks the server id, waiting until ctx ends, and returns the function that unlocks it.
// Its error is a status of gRPC.
func (l *serverLocks) lock(ctx context.Context, id string) (unlock func(), err error) {
	l.mu.Lock()
	k := l.byID[id]
	if k == nil {
		k = &serverLock{held: make(chan struct{}, 1)}
		l.byID[id] = k
	}
	k.users++
	l.mu.Unlock()
	release := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if k.users--; k.users == 0 {
			delete(l.byID, id)
		}
	}
	select {
	case k.held <- struct{}{}:
		return func() { <-k.held; release() }, nil
	case <-ctx.Done():
		release()
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}
