package mcsmv1

import "strings"

// Slug returns the short lower-case name, e.g. "paper" for SERVER_TYPE_PAPER.
func (t ServerType) Slug() string { return slug(t.String(), "SERVER_TYPE_") }

// Proxy reports whether the type is a proxy, which connects servers to a network.
func (t ServerType) Proxy() bool {
	return t == ServerType_SERVER_TYPE_VELOCITY || t == ServerType_SERVER_TYPE_BUNGEECORD
}

// Slug returns the short lower-case name, e.g. "running" for SERVER_STATE_RUNNING.
func (s ServerState) Slug() string { return slug(s.String(), "SERVER_STATE_") }

// ParseServerType returns the type with the given slug, or SERVER_TYPE_UNSPECIFIED.
func ParseServerType(slug string) ServerType {
	return ServerType(ServerType_value["SERVER_TYPE_"+strings.ToUpper(slug)])
}

func slug(name, prefix string) string { return strings.ToLower(strings.TrimPrefix(name, prefix)) }
