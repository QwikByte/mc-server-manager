package noryxv1

// Limits of HashFiles, which master and agent both check. A call stays below the 4 MiB that
// gRPC accepts by default, and no file is hashed that is larger than any file of a modpack.
const (
	MaxHashPaths  = 1000
	MaxHashedSize = 256 << 20
)
