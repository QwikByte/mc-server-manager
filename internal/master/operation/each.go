package operation

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// PerNode is how many servers of a node an action on many servers handles at a time, so
// that e.g. starting all its servers doesn't overwhelm the node.
const PerNode = 8

// errLeft is why Each left a server out.
var errLeft = httpapi.Errorf(http.StatusConflict, "The operation was cancelled before it got to this server.")

// Each calls fn for each server, given by the node it is on: servers of different nodes at
// the same time, and up to PerNode of the same node. The servers it finished are the
// progress of the operation. Once the operation is cancelled, it begins no more servers and
// calls left for them instead, if set, with the error to report. The servers it began
// finish, as an action cut short, e.g. a restart, could leave a server stopped.
func Each(ctx context.Context, nodes []string, fn func(ctx context.Context, i int), left func(i int, err error)) {
	slots := map[string]chan struct{}{}
	for _, n := range nodes {
		if slots[n] == nil {
			slots[n] = make(chan struct{}, PerNode)
		}
	}
	total := int64(len(nodes))
	var finished atomic.Int64
	Count(ctx, 0, total, "servers")
	r := from(ctx)
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Go(func() {
			slots[n] <- struct{}{}
			defer func() {
				<-slots[n]
				Count(ctx, finished.Add(1), total, "servers")
			}()
			switch {
			case r == nil:
				fn(ctx, i)
			case errors.Is(context.Cause(ctx), errCancelled):
				r.update(func(*Operation) { r.e.skipped = true })
				if left != nil {
					left(i, errLeft)
				}
			default:
				began, stop := detach(ctx)
				defer stop()
				fn(began, i)
			}
		})
	}
	wg.Wait()
}

// detach returns a context with the values and the deadline of ctx that cancelling the
// operation doesn't reach.
func detach(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(context.WithoutCancel(ctx), deadline)
	}
	return context.WithCancel(context.WithoutCancel(ctx))
}
