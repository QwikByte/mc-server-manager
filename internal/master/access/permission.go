// Package access decides what users of the panel may do. Users get permissions through
// groups; a group's node and server permissions apply to all servers or only to the nodes
// and servers of its scope. The built-in Administrators group has every permission.
// Every API route states the permission it needs when it is registered with Mux.
package access

import "slices"

// Permission allows an action in the panel.
type Permission string

const (
	NodesView         Permission = "nodes.view"
	NodesEdit         Permission = "nodes.edit"
	NodesCertificates Permission = "nodes.certificates"
	NodesDelete       Permission = "nodes.delete"
	NodesEnroll       Permission = "nodes.enroll"

	ServersView     Permission = "servers.view"
	ServersCreate   Permission = "servers.create"
	ServersStart    Permission = "servers.start"
	ServersStop     Permission = "servers.stop"
	ServersRestart  Permission = "servers.restart"
	ServersSettings Permission = "servers.settings"
	ServersDelete   Permission = "servers.delete"

	ConsoleView     Permission = "console.view"
	ConsoleCommands Permission = "console.commands"

	FilesRead  Permission = "files.read"
	FilesWrite Permission = "files.write"
	Properties Permission = "properties.edit"
	Plugins    Permission = "plugins.manage"

	BackupsView    Permission = "backups.view"
	BackupsCreate  Permission = "backups.create"
	BackupsRestore Permission = "backups.restore"
	BackupsDelete  Permission = "backups.delete"

	NetworksView     Permission = "networks.view"
	NetworksManage   Permission = "networks.manage"
	TemplatesView    Permission = "templates.view"
	TemplatesManage  Permission = "templates.manage"
	BackupJobsView   Permission = "backupjobs.view"
	BackupJobsManage Permission = "backupjobs.manage"
	PoliciesView     Permission = "policies.view"
	PoliciesManage   Permission = "policies.manage"

	LogsView Permission = "logs.view"

	SettingsView Permission = "settings.view"
	SettingsEdit Permission = "settings.edit"
	Terminal     Permission = "terminal.use"
	UsersView    Permission = "users.view"
	UsersManage  Permission = "users.manage"
	GroupsManage Permission = "groups.manage"
)

// Administrators is no permission but the built-in group, for what only its members may do,
// e.g. updating MC Server Manager. Groups can't be given it.
const Administrators Permission = "administrators"

// Info describes a permission for the panel.
type Info struct {
	ID          Permission `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	// Scoped permissions only apply to the nodes and servers in the scope of a group;
	// the others apply everywhere.
	Scoped bool `json:"scoped"`
	// Requires are granted along with the permission, e.g. seeing the servers one may start.
	Requires []Permission `json:"requires,omitempty"`
}

// Area is a part of the panel and the permissions for it.
type Area struct {
	Name        string `json:"name"`
	Permissions []Info `json:"permissions"`
}

func scoped(id Permission, label, description string, requires ...Permission) Info {
	return Info{id, label, description, true, requires}
}

func global(id Permission, label, description string, requires ...Permission) Info {
	return Info{id, label, description, false, requires}
}

// Catalog lists all permissions by area.
var Catalog = []Area{
	{"Nodes", []Info{
		scoped(NodesView, "See nodes", "Their machines, agents and storage locations."),
		scoped(NodesEdit, "Change nodes", "Name, agent address, default storage, port range and memory limit.", NodesView),
		scoped(NodesCertificates, "Renew certificates", "Renew the certificate of a node before it is due.", NodesView),
		scoped(NodesDelete, "Remove nodes", "The panel stops managing them; their servers keep running.", NodesView),
		scoped(NodesEnroll, "Add nodes", "Add nodes and create join tokens for agents. Only in groups for all servers, as new nodes are outside other scopes.", NodesView),
	}},
	{"Servers", []Info{
		scoped(ServersView, "See servers", "Servers with their state, settings and plugins."),
		scoped(ServersCreate, "Create servers", "On whole nodes of the scope, also from templates; their plugins also need the permission to manage plugins.", ServersView),
		scoped(ServersStart, "Start servers", "", ServersView),
		scoped(ServersStop, "Stop servers", "", ServersView),
		scoped(ServersRestart, "Restart servers", "", ServersView),
		scoped(ServersSettings, "Change server settings", "Name, version, memory, port, Java and CPU limit. The server restarts.", ServersView),
		scoped(ServersDelete, "Delete servers", "With all their data and backups.", ServersView),
	}},
	{"Console", []Info{
		scoped(ConsoleView, "Read the console", "The live output of servers.", ServersView),
		scoped(ConsoleCommands, "Send console commands", "Run any command of the server, e.g. op.", ConsoleView),
	}},
	{"Files and configuration", []Info{
		scoped(FilesRead, "Browse and download files", "Secrets such as the RCON password stay hidden. Also needed to duplicate a server, as the copy contains its files.", ServersView),
		scoped(FilesWrite, "Change files", "Upload, edit, move and delete files. Uploaded plugins run with the server and can read its secrets, such as the forwarding secret of its network.", FilesRead),
		scoped(Properties, "Edit server.properties", "", ServersView),
		scoped(Plugins, "Manage plugins and mods", "Install, update, upload and remove them. They run with the server and can read its secrets.", ServersView),
	}},
	{"Backups", []Info{
		scoped(BackupsView, "See and download backups", "Downloads contain all backed up files, without secrets such as the RCON password.", ServersView),
		scoped(BackupsCreate, "Back up servers", "", BackupsView),
		scoped(BackupsRestore, "Restore backups", "Replaces the backed up data; a running server restarts.", BackupsView),
		scoped(BackupsDelete, "Delete backups", "", BackupsView),
	}},
	{"Networks and templates", []Info{
		global(NetworksView, "See networks", ""),
		global(NetworksManage, "Manage networks", "Create, change and delete networks, which configures and restarts their servers.", NetworksView),
		global(TemplatesView, "See templates", "Creating a server from a template also needs the permission to create servers."),
		global(TemplatesManage, "Manage templates", "Create, change and delete templates.", TemplatesView),
	}},
	{"Backup jobs and policies", []Info{
		global(BackupJobsView, "See backup jobs", ""),
		global(BackupJobsManage, "Manage backup jobs", "Create, change, delete and run backup jobs for any server.", BackupJobsView),
		global(PoliciesView, "See policies", ""),
		global(PoliciesManage, "Manage policies", "Create, change, delete and run policies, which restart, stop and start any server or run console commands.", PoliciesView),
	}},
	{"Logs", []Info{
		scoped(LogsView, "See logs", "Actions, warnings and errors of the master and the agents. Entries about users, groups, settings and the master itself need it for all servers."),
	}},
	{"System", []Info{
		global(SettingsView, "See the master's settings", "Its version, addresses and settings."),
		global(SettingsEdit, "Change the master's settings", "Enrollment address, join tokens, sessions and limits of new nodes.", SettingsView),
		global(Terminal, "Use the terminal", "Each command also needs its own permission, e.g. to restart a server."),
	}},
	{"Users and groups", []Info{
		global(UsersView, "See users and groups", ""),
		global(UsersManage, "Manage users", "Invite, disable and delete users, create setup links and choose their groups. Only for users without more permissions than oneself.", UsersView),
		global(GroupsManage, "Manage groups", "Create, change and delete groups. Only with permissions one has oneself.", UsersView),
	}},
}

// lookup finds the description of a permission.
func lookup(p Permission) (Info, bool) {
	for _, a := range Catalog {
		if i := slices.IndexFunc(a.Permissions, func(info Info) bool { return info.ID == p }); i >= 0 {
			return a.Permissions[i], true
		}
	}
	return Info{}, false
}

// withRequired adds the permissions that the given ones require, sorted and without duplicates.
func withRequired(perms []Permission) []Permission {
	all := slices.Clone(perms)
	for i := 0; i < len(all); i++ {
		info, _ := lookup(all[i])
		for _, r := range info.Requires {
			if !slices.Contains(all, r) {
				all = append(all, r)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}
