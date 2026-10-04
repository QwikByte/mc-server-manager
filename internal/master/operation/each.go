package operation

import (
	"context"
	"sync"
	"sync/atomic"
)

// PerNode is how many servers of a node an action on many servers handles at a time, so
// that e.g. starting all its servers doesn't overwhelm the node.
const PerNode = 8

// Each calls fn for each server, given by the node it is on: servers of different nodes at
// the same time, and up to PerNode of the same node. The servers it finished are the
// progress of the operation.
func Each(ctx context.Context, nodes []string, fn func(i int)) {
	slots := map[string]chan struct{}{}
	for _, n := range nodes {
		if slots[n] == nil {
			slots[n] = make(chan struct{}, PerNode)
		}
	}
	total := int64(len(nodes))
	var finished atomic.Int64
	Count(ctx, 0, total, "servers")
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Go(func() {
			slots[n] <- struct{}{}
			defer func() {
				<-slots[n]
				Count(ctx, finished.Add(1), total, "servers")
			}()
			fn(i)
		})
	}
	wg.Wait()
}
