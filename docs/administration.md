# Settings, users and permissions

## Settings

The **Settings** page configures the master, gives an overview of the agents and manages who may do what. Each tab only
shows to users with the permission for it.

### General

The **General** tab shows the running master (version, uptime, addresses, CA fingerprint, certificates) and its
settings. The address the panel listens at (it replaces `--http-addr`; empty uses the flag again) and its HTTPS apply
when the master starts again, e.g. with **Restart master** or the next update; the page says so until then. Only
administrators change them, as they can open the panel to other networks. The address is checked when saved, and if the
master can't listen there when it starts, e.g. because another program took the port, the panel falls back to
`--http-addr` and shows why, so a wrong address can't lock you out. For HTTPS, the panel serves a self-signed
certificate for the IP addresses and names of the machine, the domain if one is set, and the host of the enrollment
address; it shows its fingerprint to compare with the browser's warning. Or it gets one from Let's Encrypt for its
domain, which has to point to the machine: Let's Encrypt checks it at port 443 of the panel or at port 80, where the
master then sends browsers to HTTPS. Until Let's Encrypt issued one, and for other names such as an IP address, the
panel serves the self-signed certificate, and the page shows why. With a certificate of the command line (`--tls-cert`),
these settings don't apply. **Restart master**, for administrators, stops the master and lets systemd start it again
(also reading `master.env` again), unless servers are moving to another node; Minecraft servers keep running. The other
settings apply right away: the enrollment address join tokens contain (it replaces `--public-enroll-addr`; empty uses
the flag again), how long join tokens are valid (5 minutes to a day, 1 hour by default), how long sign-ins to the panel
last (1 hour to a week, 12 hours by default), how long log entries are kept (1 day to a year, 30 days by default) and
how much space the log may take ([100 MiB to 100 GiB, 2 GiB by default](monitoring.md#logs)), the
port range and memory reserve that new nodes get, and whether the master looks for updates. Administrators can also look
for an update right away.

### Agents

The **Agents** tab lists all nodes with their agent version, certificate and settings, which can be changed there too.
Administrators update an agent that is older than the master there with **Update** (see [Updates](installation.md#updates)).

### Terminal

The **Terminal** runs the commands of `noryx-agent` (`status`, `server …`, `backup …`, `datastore …`) on any node, and
the master's own commands: `status`, `node list`, `node renew <node>` and `logs`. `help` lists them; output streams in
as it happens, e.g. for `server logs <id>`, and Ctrl+C stops a command. Commands that write nothing for a while, e.g.
`server stop <id>` with a long stop timeout, keep the connection alive, so that a reverse proxy in front of the master
doesn't cut them off. A node's page opens its terminal directly.
Besides the permission to use the terminal, every command needs its own, e.g. `server restart <id>` the one to restart
this server.

Tab completes the commands the user may run, their flags and their arguments: IDs of servers, also from the first
letters of their names, and of nodes, datastores, databases and backups. When several choices are left, a second Tab
lists them, e.g. servers with their names, and a click takes one. Tab in an empty line moves on to the next control.

↑ and ↓ repeat earlier commands, which the browser tab keeps for each node and the master until it is closed or the
user signs out.

### Users and groups

The **Users** tab invites users, chooses their groups, disables and deletes them, creates setup links and turns off
two-factor authentication for users who lost their phone. The **Groups** tab defines what their members may do, see
[Users and permissions](#users-and-permissions).

## Users and permissions

Users get their permissions from groups; a user can be in several groups and has the permissions of all of them.

### Fine-grained permissions

There are permissions for every action, by area: nodes (see, change, renew certificates, remove, add, manage the private
network, which applies to all nodes), servers (see, create, start, stop, restart, change settings, delete), console
(read, send commands), players (kick, ban, whitelist and make operators), files and configuration (browse and download,
change files, `server.properties`, plugins and mods), backups (see and download, back up, restore, delete), the log,
networks, databases of networks, templates, file sets, backup jobs, schedules, the master's settings, the terminal,
users and groups. Previewing and applying a file set also needs the permission to change the files of every server it
touches, and restarting them the one to restart each. Choosing a permission also chooses what it needs, e.g. seeing the
servers one may restart.

### Scopes

The node and server permissions of a group apply to all servers, or only to chosen nodes (including servers created
later) and single servers, e.g. a group that may restart the lobby and use its console. Lists only show the nodes and
servers a user may see, and the log only the entries about them; entries about the master, users or groups need the
permission for all servers. Other permissions, e.g. for networks or schedules, apply everywhere, because they act on any
server.

### Administrators

The built-in Administrators group has every permission, also those that later versions add. Only its members see and
install updates; no permission allows that to other groups. `noryx-master user add` creates administrators, e.g. the
first one or after losing access.

### Invitations

New users get a setup link (valid for three days, usable once) to choose their password; the same link resets a
forgotten password. The token is in the link's fragment, which browsers don't send to servers, and the panel removes it
from the address bar once it was read. Users change their own password on their account page (in the menu of their name
in the sidebar), which signs them out everywhere else.

### Two-factor authentication

Two-factor authentication is off until users set it up on their account page: they scan a QR code with an authenticator
app (TOTP, e.g. Google Authenticator, Aegis or a password manager), confirm with a code of it and their password, and
get 10 recovery codes that each replace a code once. Then signing in asks for a code after the password, and other
sessions end. A setup link then only sets the password; signing in still needs a code. Turning it off and new recovery
codes need the password. Users who may manage a user turn it off for them, e.g. after they lost their phone and recovery
codes, but not for themselves; without any administrator who can still sign in, `noryx-master user add` creates a new
one.
