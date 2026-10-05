package noryxv1

// ContainerMemoryMB is the hard memory limit of the container of a server whose heap is
// heapMB: Java needs memory beyond its heap, e.g. for its code and threads. The master
// counts it against the memory of a node.
func ContainerMemoryMB(heapMB uint32) int64 { return int64(heapMB)*5/4 + 256 }

// MaxHeapMB is the largest heap whose container fits in containerMB, the inverse of
// ContainerMemoryMB.
func MaxHeapMB(containerMB int64) int64 { return max(0, (containerMB-256)*4/5) }
