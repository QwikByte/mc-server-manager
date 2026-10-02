package access

import (
	"context"
	"maps"
	"slices"
)

// Target is a server, or all servers of a node (including later ones) if ServerID is empty.
type Target struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

// scope is where a permission applies: everywhere, or on whole nodes and single servers.
type scope struct {
	all     bool
	nodes   map[string]bool
	servers map[Target]bool
}

// covers reports whether s includes all of o.
func (s *scope) covers(o *scope) bool {
	if s.all {
		return true
	}
	if o.all {
		return false
	}
	for n := range o.nodes {
		if !s.nodes[n] {
			return false
		}
	}
	for t := range o.servers {
		if !s.nodes[t.NodeID] && !s.servers[t] {
			return false
		}
	}
	return true
}

// Grants are the permissions of a user: the union of those of the user's groups.
type Grants struct {
	admin bool
	perms map[Permission]*scope
}

// Admin returns the grants of the Administrators group.
func Admin() Grants { return Grants{admin: true} }

// add grants p in the given targets, or everywhere if all is set or p isn't scoped.
func (g *Grants) add(p Permission, all bool, targets []Target) {
	if g.perms == nil {
		g.perms = map[Permission]*scope{}
	}
	s := g.perms[p]
	if s == nil {
		s = &scope{nodes: map[string]bool{}, servers: map[Target]bool{}}
		g.perms[p] = s
	}
	info, _ := lookup(p)
	if all || !info.Scoped {
		s.all = true
		return
	}
	for _, t := range targets {
		if t.ServerID == "" {
			s.nodes[t.NodeID] = true
		} else {
			s.servers[t] = true
		}
	}
}

// merge adds the grants of o to g.
func (g *Grants) merge(o Grants) {
	g.admin = g.admin || o.admin
	for p, s := range o.perms {
		g.add(p, s.all, nil)
		for n := range s.nodes {
			g.add(p, false, []Target{{NodeID: n}})
		}
		g.add(p, false, slices.Collect(maps.Keys(s.servers)))
	}
}

// Has reports whether p applies everywhere.
func (g Grants) Has(p Permission) bool {
	return g.admin || g.perms[p] != nil && g.perms[p].all
}

// On reports whether p applies to a server, or to a whole node if serverID is empty.
func (g Grants) On(p Permission, nodeID, serverID string) bool {
	s := g.perms[p]
	return g.admin || s != nil && (s.all || s.nodes[nodeID] || serverID != "" && s.servers[Target{nodeID, serverID}])
}

// Somewhere reports whether p applies anywhere on a node: to the node or to one of its
// servers. An empty nodeID asks for any node.
func (g Grants) Somewhere(p Permission, nodeID string) bool {
	s := g.perms[p]
	switch {
	case g.admin:
		return true
	case s == nil:
		return false
	case nodeID == "" || s.all || s.nodes[nodeID]:
		return true
	}
	for t := range s.servers {
		if t.NodeID == nodeID {
			return true
		}
	}
	return false
}

// Scope returns where p applies: everywhere, or on whole nodes and single servers.
func (g Grants) Scope(p Permission) (all bool, nodes []string, servers []Target) {
	s := g.perms[p]
	switch {
	case g.admin:
		return true, nil, nil
	case s == nil:
		return false, nil, nil
	}
	return s.all, slices.Collect(maps.Keys(s.nodes)), slices.Collect(maps.Keys(s.servers))
}

// SeesNode reports whether the node is shown: if the user may see it or some of its servers.
func (g Grants) SeesNode(nodeID string) bool {
	return g.Somewhere(NodesView, nodeID) || g.Somewhere(ServersView, nodeID)
}

// Covers reports whether g includes every permission of o in at least o's scope. Users may
// only grant what they have, and only manage users who have no more than they do.
func (g Grants) Covers(o Grants) bool {
	if g.admin {
		return true
	}
	if o.admin {
		return false
	}
	for p, s := range o.perms {
		if own := g.perms[p]; own == nil || !own.covers(s) {
			return false
		}
	}
	return true
}

// view is how the panel learns the permissions of the signed-in user.
type view struct {
	Admin       bool                     `json:"admin"`
	Permissions map[Permission]scopeView `json:"permissions"`
}

type scopeView struct {
	All     bool     `json:"all"`
	Nodes   []string `json:"nodes"`
	Servers []Target `json:"servers"`
}

func (g Grants) view() view {
	v := view{Admin: g.admin, Permissions: map[Permission]scopeView{}}
	for p, s := range g.perms {
		// Empty lists are sent as such rather than as null.
		v.Permissions[p] = scopeView{s.all, slices.AppendSeq([]string{}, maps.Keys(s.nodes)), slices.AppendSeq([]Target{}, maps.Keys(s.servers))}
	}
	return v
}

type grantsKey struct{}

// WithGrants returns a context carrying the grants of the user it acts for.
func WithGrants(ctx context.Context, g Grants) context.Context {
	return context.WithValue(ctx, grantsKey{}, g)
}

// From returns the grants of the user a request acts for; without any, nothing is allowed.
func From(ctx context.Context) Grants {
	g, _ := ctx.Value(grantsKey{}).(Grants)
	return g
}
