package noryxv1

import "strings"

// RuntimeUnavailable is the runtime that GetInfo reports while the agent can't reach it.
const RuntimeUnavailable = "unavailable"

// ReasonRuntimeUnavailable is the reason of the ErrorInfo that agents attach to Unavailable
// errors of calls that failed because the container runtime can't be reached. Its metadata
// names the runtime under MetadataRuntime, which older agents leave out.
const ReasonRuntimeUnavailable = "RUNTIME_UNAVAILABLE"

// MetadataRuntime is the key of the runtime's name in the metadata of ReasonRuntimeUnavailable.
const MetadataRuntime = "runtime"

// The container runtimes an agent can run servers with.
const (
	RuntimeDocker = "docker"
	RuntimePodman = "podman"
)

// RuntimeTitle returns how texts name a runtime, e.g. "Podman"; older agents name none, as
// they only run Docker.
func RuntimeTitle(name string) string {
	if name == RuntimePodman {
		return "Podman"
	}
	return "Docker"
}

// RuntimeOf returns the name and version of the runtime the agent runs servers with; the
// version is empty while the agent can't reach it. Older agents only tell "docker <version>".
func (r *GetInfoResponse) RuntimeOf() (name, version string) {
	if name = r.GetRuntimeName(); name != "" {
		return name, r.GetRuntimeVersion()
	}
	version, _ = strings.CutPrefix(r.GetRuntime(), RuntimeDocker+" ")
	if version == RuntimeUnavailable {
		version = ""
	}
	return RuntimeDocker, version
}
