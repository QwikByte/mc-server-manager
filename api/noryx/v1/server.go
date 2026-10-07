package noryxv1

import "time"

// ContainerMemoryMB is the hard memory limit of the container of a server whose heap is
// heapMB: Java needs memory beyond its heap, e.g. for its code and threads. The master
// counts it against the memory of a node.
func ContainerMemoryMB(heapMB uint32) int64 { return int64(heapMB)*5/4 + 256 }

// MaxHeapMB is the largest heap whose container fits in containerMB, the inverse of
// ContainerMemoryMB.
func MaxHeapMB(containerMB int64) int64 { return max(0, (containerMB-256)*4/5) }

// A server gets its stop timeout to stop gracefully, e.g. to save its worlds, before it is
// killed. Whoever waits for a stop allows for the longest.
const (
	DefaultStopTimeout = time.Minute
	MinStopTimeout     = 30 * time.Second
	MaxStopTimeout     = 10 * time.Minute
)

// StopTimeout is the stop timeout of a server with stop_timeout_seconds; 0, as older masters
// and agents send it, means DefaultStopTimeout.
func StopTimeout(seconds uint32) time.Duration {
	if seconds == 0 {
		return DefaultStopTimeout
	}
	return time.Duration(seconds) * time.Second
}
