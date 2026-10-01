// Package buildinfo exposes metadata injected at build time.
package buildinfo

// Version is set via -ldflags "-X github.com/QwikByte/mc-server-manager/internal/buildinfo.Version=v1.0.0".
var Version = "dev"
