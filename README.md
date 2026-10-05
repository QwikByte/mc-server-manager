# Noryx

All-in-one management for Minecraft servers and whole networks. One **master** with an admin panel
controls **agents** on any number of dedicated servers.

```
                                   ┌──────────────── node: dedicated server ───────────────┐
 Browser ──HTTPS──▶ noryx-master ──┼─gRPC + mTLS──▶ noryx-agent ─▶ Docker: Paper, Velocity…│
 (admin panel)      panel, REST API│                  ▲                                     │
                    enrollment     │  local CLI ──Unix socket                               │
                    SQLite, CA     └────────────────────────────────────────────────────────┘
```

| Program        | Runs on              | Responsibility                                                                                     |
| -------------- | -------------------- | -------------------------------------------------------------------------------------------------- |
| `noryx-master` | the panel host       | Admin panel (embedded), REST API, node registry, certificate authority, enrollment endpoint        |
| `noryx-agent`  | every dedicated host | Runs the Minecraft servers through a runtime (Docker), accepts commands from the master or its CLI |

Servers run as containers based on [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server)
(Vanilla, Paper, Purpur, Folia, Leaf, Fabric, Quilt, Forge, NeoForge) and [itzg/mc-proxy](https://github.com/itzg/docker-mc-proxy)
(Velocity, BungeeCord, Waterfall). Container labels are the agent's only state, so servers keep running while an agent restarts.

Each server has a live console in the panel: its output streams in as it happens, and commands go to game servers
through the RCON connection the server image provides, and to proxies through their own console, whose answer follows
in the output. Proxies created by earlier versions accept commands once they were created again, e.g. by saving their
settings.

The file manager of a server browses its data, uploads files by drag and drop (up to 16 GB each, streamed through
the master, as long as 1 GB stays free on the node, like for backups), edits configuration files in the browser and downloads files or whole folders as ZIP archives.

`server.properties` can be edited as a form: grouped settings with switches, choices and validated numbers, a
MOTD editor with colour codes and preview, and a search. Only properties of the server's Minecraft version are
shown, comments in the file are kept, and properties the manager relies on (container port, RCON) are locked.

Secrets such as the RCON password and the forwarding secret of a network never reach the panel. The file manager
hides files that only hold secrets (`.rcon-cli.env`, `.rcon-cli.yaml`, `forwarding.secret`) and shows
`server.properties`, `config/paper-global.yml` and the configuration of the forwarding mods of networks with their
secrets as `<hidden>`, which saving keeps. Downloads of
folders and backups leave them out the same way. Plugins and mods run with the server, though, and can read them.

The settings of a server can be changed after it was created: name, Minecraft version, the version of the mod loader
of Fabric, Quilt, Forge and NeoForge servers (the newest unless set), memory, port, Java version (8, 11, 17, 21, 25 or
the newest), when it starts on its own, Aikar's flags, JVM options and a CPU limit.
The agent creates the container again with the same data; the old container is only removed once the new one
exists. A server keeps the image it was created with; **Update image** in its settings pulls the newest one and, if it
changed, creates the container again the same way. The old image is removed once no server uses it.

A server that crashed and starts again shows as **crashing**, with how often it crashed and its exit code. After 5
crashes in a row, each within 10 minutes of its start, the agent stops it, as Docker would start it again forever.

A server can be duplicated on its node: the copy gets all files, worlds and settings under a new name and port, and
starts stopped. A running game server first writes its worlds to disk and pauses saving while they are copied, so
players stay connected. The copy doesn't take over the original's place in a network.

A server can move to another node with its ID, files, settings and, if chosen, its backups; otherwise the backups are
deleted with it. Port, storage location, memory and CPU limits are checked on the new node first. The server then
stops, its data is copied through the master, and it starts on the new node if it ran before. Its backup jobs,
policies, the scopes of groups and its usage history follow it, and its network is configured again, which restarts
the proxy. The original is deleted only once the server is complete on the new node; if anything fails before, the
copy goes away and the server runs where it was. While it moves, the panel shows the progress, refuses changes to the
server and continues on the new node once it is done; scheduled tasks leave it out meanwhile. The new node needs free
space for the archive of the data besides the data itself, until it is extracted. If the master stops during a move,
the server stays on its old node, stopped.

Each node has settings for its servers: the storage location preselected for new servers, a port range (new
servers get the first free port in it) and a memory limit, so that servers together can't get more memory than
the node has minus a reserve for the system (1 GB unless changed). Name and agent address can be changed too.

The **Overview** is the panel's start page: the players online, the servers by state, the nodes with what they use,
the networks, the servers with the most players, and what needs attention: crashing servers, offline nodes, nodes with
more memory assigned than they can give or almost full storage, and proxies that are stopped while their servers run.

The **Servers** page and the page of each node list servers as cards or as a compact table, the table from 13 servers
on until one is chosen. They are searched, filtered by state, type, node, network and tag, sorted by name, state,
players, CPU, memory or node, and grouped by network, node, type or tag, in groups that fold away. The address keeps
all of it, so that a view can be shared or bookmarked. Selected servers start, restart or stop together, run a console
command such as `save-all`, or get and lose tags; an action applies to the selected servers in a fitting state on which
the user may do it, at most 8 at a time on each node, and the panel tells which failed.

Servers have **tags** such as `lobby` or `bedwars`: up to 10, each of up to 24 letters, digits, `-` and `_`. The master
keeps them; they follow a server that moves, copies get them, and they go with a deleted server. Changing them needs the
permission to change the server's settings, though it doesn't restart the server.

**Ctrl+K** (⌘K) searches servers, also by tag, networks, nodes and pages from anywhere in the panel. A search that starts
with an action, e.g. `restart lobby`, starts, restarts or stops a server or opens its console.

Long actions run as **operations**: creating, copying and changing servers, updating their image, installing plugins
on many servers, backing up and restoring, and the actions on networks and on many servers at once. The panel shows
their steps as they go, e.g. how much of a server image is downloaded or how many servers of a network are configured.
A dialog can't be closed by mistake meanwhile; **Continue in the background** hands the operation to a notification,
which follows it to its end and links to its result. The operations of the last hour, also those of other users, are in
the list behind the button next to the warnings. The master runs them in the background: an answer comes right away if
the action ends within a second, otherwise `202 Accepted` with the operation, which `GET /api/operations/{id}` follows,
so that neither a closed browser nor a proxy in front of the master cuts it off. The master can't restart while one runs.
Agents tell the progress of their part, e.g. the bytes of a download, through `ProgressService`; agents of older versions
only let the panel show the steps.

The panel speaks English and German. It follows the browser until someone chooses a language with the button next to the
colour theme, which the panel stores for the signed-in user, so that it applies in all their browsers; on the sign-in
page, the choice applies to the browser. Dates, times and numbers follow the language too. What the master and the
agents send, such as errors, the log and the descriptions of permissions, stays English.

## Installation

Releases contain packages for Debian, Ubuntu and their derivatives (`.deb`), Fedora, RHEL, Rocky Linux, AlmaLinux and
openSUSE (`.rpm`) and Arch Linux, each for x86_64 and arm64. Only current Linux systems with systemd and OpenSSL 3 are
supported, e.g. Debian 12, Ubuntu 22.04, RHEL, Rocky Linux and AlmaLinux 9, openSUSE Leap 16 and newer. The installer
picks the package, checks the signature of the release and the package's checksum and sets everything up. Running it
again updates, and `--version vX.Y.Z` installs a certain release. While another installation of packages runs, e.g. the
automatic updates of a new server, it waits up to 10 minutes and says so. It asks questions only when started from a
file as below; piped to `sudo bash`, it uses the defaults instead, because `sudo-rs`, the `sudo` of newer Ubuntu
releases, doesn't pass on what is typed then.

**Master**, the panel:

```sh
curl -fsSLO https://github.com/QwikByte/noryx/releases/latest/download/install.sh && sudo bash install.sh master
```

It asks for the host name or IP address under which the nodes reach this machine (`--public-host`), for the IP address
and port the panel listens at (`--panel-addr`, `127.0.0.1:8080` by default, `0.0.0.0:<port>` for all interfaces, ports
from 1024 on), and for the password of the first administrator, `admin` unless `--admin` names another (piped, it
generates one and writes it to `/etc/noryx/admin-password`, which only root can read). The password needs at least 12 characters and isn't shown while you type it. These options
only apply to a new installation; later, both addresses can be changed in the panel's settings.
Open port 9443 for the nodes. For plugins and mods, the master needs HTTPS access to `api.modrinth.com` and
`cdn.modrinth.com`. Browsers only sign in over HTTPS or at `localhost`. Until the panel serves HTTPS, open it through an
SSH tunnel, e.g. `ssh -L 8080:127.0.0.1:8080 <user>@<master>` and `http://localhost:8080`. Then either turn on HTTPS
under **Settings → General**: a certificate of Let's Encrypt for a domain, or a self-signed one, which also works for IP
addresses but makes browsers warn. Or serve the panel with a reverse proxy, e.g. with [Caddy](https://caddyserver.com)
and this `Caddyfile`, which also gets the certificate:

```
panel.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

Then add `--trusted-proxy 127.0.0.1` to `NORYX_MASTER_OPTS` in `/etc/noryx/master.env`, so that the master takes the
address of each client from the proxy's `X-Forwarded-For` header. Otherwise all clients share the proxy's address, and
with it the budget of the sign-in rate limit, and the log shows only the proxy's address.

**Nodes.** Add a node in the panel under **Nodes**. It shows a command that installs the agent in the master's version,
offers to install Docker if it's missing (`--install-docker` doesn't ask), connects the agent with a join token and
starts it:

```sh
curl -fsSLO https://github.com/QwikByte/noryx/releases/download/<version>/install.sh && sudo bash install.sh agent --join <join-token>
```

Allow port 7443 only from the master's IP address.

**Everything on one machine.** `sudo bash install.sh all` installs master and agent, registers the machine as node and
connects its agent, which then only accepts connections from the machine itself.

**Updates.** The master looks for a new release every 6 hours. Administrators then see a notice in the panel with the
release notes and install it with **Install now**: the master installs the release, restarts on it, and then updates
every agent to its version, also the one on its own machine. Running services restart; Minecraft servers keep running.
Agents that were offline are listed with a button to update them later. The check can be turned off in the settings,
e.g. for a master without internet access. While GitHub's API limits the requests of the master's IP address (60 an
hour, shared with everything behind the same address), the master reads only the version from the release page, and
the panel links to the release notes on GitHub. On the command line, `… | sudo bash -s -- update` updates what is
installed; update the master first, then the nodes.

| Where                                           | What                                                                                           |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `/etc/noryx/master.env`, `/etc/noryx/agent.env` | Options of the services, e.g. listen addresses or a log file; `systemctl restart` applies them |
| `/var/lib/noryx-master`, `/var/lib/noryx-agent` | Database and CA of the master; credentials, server data and backups of the agent               |
| `journalctl -u noryx-master`, `-u noryx-agent`  | What the services log; `--log-format json` in the options suits log collectors                 |

**Commands on the master's host** run as the master's user `noryx`: `sudo -u noryx noryx-master logs` shows the log,
`… user add <name>` creates an administrator, e.g. after losing access,
`… node add <name> <agent-address> --public-enroll-addr <host:port>` adds a node and prints its join token for scripts,
and `… backup <file>` saves the master's database and CA (see [Backups](#backups)).

**Commands on a node:** `sudo noryx-agent status` checks the node, `server logs <id>` follows a console and `logs -f` the
agent's own log. `backup list <id>`, `backup create <id>` and `backup restore <id> <backup-id>` work while the master is
unreachable too. `storage add ssd /mnt/ssd/noryx` allows another directory for server data, e.g. on a faster disk; new
servers can then be created there from the panel, and backup jobs can keep their backups there.

**Removing.** `apt remove`, `dnf remove` or `pacman -R` with `noryx-master` or `noryx-agent` stops and removes a program
but keeps its data. Delete `/var/lib/noryx-master`, `/var/lib/noryx-agent` and `/etc/noryx` to remove that too. The
Minecraft servers of a node keep running in Docker; delete them in the panel before.

**By hand.** Each release also has `.tar.gz` archives with the static binary, its systemd unit and its options, for
other distributions: the unit expects the binary in `/usr/bin`, the options in `/etc/noryx` and, for the master, a system
user `noryx`. `checksums.txt` lists the SHA-256 checksums of all files, and
`gh attestation verify <file> --repo QwikByte/noryx` proves that a file was built by the release workflow.
`checksums.txt.sig` is the signature of the checksums, which the installer checks with the release key. To check files
by hand, e.g. `install.sh` before the first installation, download `checksums.txt` and `checksums.txt.sig` too:

```sh
printf -- '-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEA18ilyBW0qkWpfEqFR+rW5eQeGC3Sif4OiD8RKpEry5s=\n-----END PUBLIC KEY-----\n' > noryx-release.pem
openssl pkeyutl -verify -pubin -inkey noryx-release.pem -rawin -in checksums.txt -sigfile checksums.txt.sig
sha256sum -c --ignore-missing checksums.txt
```

## Templates

A template preconfigures new servers: software, Minecraft version, memory, the settings above, `server.properties`
and a list of plugins or mods from Modrinth. When a server is created from a template, only the node, name, port and
storage are chosen; `server.properties` is written before the first start and each plugin is installed in the newest
release that suits the server, so templates don't go stale. Templates are created from scratch or from an existing
server ("Save as template"), which takes its settings, its properties (except those the manager sets) and the plugins
that come from Modrinth. Worlds and plugin configurations are not part of templates.

## Plugins and mods

Plugins (Paper, Purpur, Folia, Leaf, Velocity, BungeeCord, Waterfall) and mods (Fabric, Quilt, Forge, NeoForge) are
installed from [Modrinth](https://modrinth.com), either on any number of servers at once from the **Plugins** page or
from the **Plugins**/**Mods** tab of a server. The **Plugins** page switches between plugins and mods, so a project made for
both only shows the software and servers of the chosen kind. The search filters by software, Minecraft version,
categories (e.g. economy, management, optimization) and, for mods, those players don't have to install, and sorts by
relevance, downloads, followers, newest or recently updated. The master picks the newest release for each server's
software and Minecraft version, installs the projects it requires, and replaces an older version of the same project.
Another version that suits the server, betas and alphas included, can be chosen instead, also to downgrade a project.
Installed files are recognised by their hash, so the tab shows their project, version and available updates, also for
files uploaded by hand; it searches, filters (updates, not from Modrinth) and sorts them, and updates all at once. Own
`.jar` files can be uploaded too. Servers load changes when they restart.

**Modpacks.** A server can be created from a [Modrinth modpack](https://modrinth.com/modpacks) for Fabric, Quilt,
Forge or NeoForge: **Create server** searches the modpacks and offers the versions of the chosen one, the newest release
first. The pack decides the software, the Minecraft version and the version of the mod loader. The master downloads the
pack, then the files servers need (not those for players only) and writes them into the new server, with the files the
pack brings itself (`overrides`, then `server-overrides`), except `server.properties` and `eula.txt`. Afterwards it is a
server like any other: the **Mods** tab recognises the mods and updates them. A server that didn't get all files of its
pack is deleted again. Creating servers from modpacks needs the permission to manage plugins and mods on the node.

## Backups

Backups are ZIP archives that the agent keeps on the server's node, in the `backups` folder of a storage location
(`<data-dir>/backups` by default). What a backup contains is chosen per backup or job: worlds (every folder with a
`level.dat`, also those added later), plugins or mods with their settings, configuration (the files in the server's
folder except jars and logs, and `config/`), everything, or further files and folders. A running game server writes
its worlds to disk first and pauses saving while they are archived, so players stay connected.

- **By hand.** The **Backups** tab of a server backs it up now, e.g. before an update, and lists, downloads, restores
  and deletes its backups.
- **Jobs.** The **Backups** page schedules backup jobs for servers or whole nodes (including servers created later):
  on chosen weekdays at one or more times of day in a time zone. A job keeps the newest backups per server and deletes
  older ones; backups made by hand are never deleted that way. A job backs up one server per node at a time.
- **Restoring** replaces what a backup contains with its backed up state: a backup of the worlds restores the worlds
  and leaves plugins and settings alone. The archive is extracted next to the data first, so a running server is only
  stopped while the files are swapped, and started again afterwards.
- Deleting a server deletes its backups too. Locally, `noryx-agent backup list|create|restore` works without the
  master, e.g. to restore a server while the master is unreachable.

**The master** keeps users, nodes, networks, templates, backup jobs, policies, settings and the log in its database, and
the certificate authority (CA) that its agents trust in `pki`. Losing them means enrolling every node again.
`sudo -u noryx noryx-master backup <file>` saves both in a `.tar.gz` archive, also while the master runs; with `-`
instead of a file, it writes the archive to stdout, e.g. for `ssh master 'sudo -u noryx noryx-master backup -' > master.tar.gz`
on another machine. The CA's private key lets anyone control the agents, so keep the archive as safe as the master. To
restore it, e.g. on a new machine after `install.sh master`:

```sh
sudo systemctl stop noryx-master
sudo rm -f /var/lib/noryx-master/master.db-wal /var/lib/noryx-master/master.db-shm
sudo tar -xzf master.tar.gz -C /var/lib/noryx-master
sudo chown -R noryx:noryx /var/lib/noryx-master
sudo systemctl start noryx-master
```

The nodes keep working with the restored master. If its IP address changed, allow the new one on port 7443 of the nodes.

## Policies

Policies rule servers or whole nodes on a schedule like that of backup jobs:

- **Restart**, e.g. every night at 4:00. Players are warned in the chat beforehand (10, 5 and 1 minutes before by
  default, with an editable message) and the servers restart at the scheduled time.
- **Stop** and **start**, e.g. for opening hours. Stopping warns the players like restarting.
- **Console command**, e.g. a broadcast every evening.

Restarts and stops only concern running servers, starts only stopped ones. The master runs backup jobs and policies;
runs it misses while it is down are skipped. The latest run and its errors are shown with each job and policy, and
both can be run right away. Deleted servers are removed from them automatically.

## Networks

A network puts game servers behind a proxy, on one node or spread across nodes. The panel doesn't add a network system
of its own: it writes the configuration the proxies and game servers document, and shows it as a picture. The master
stores the network and configures each server through its agent.

| Proxy                 | Configuration   | Forwarding                                   |
| --------------------- | --------------- | -------------------------------------------- |
| Velocity              | `velocity.toml` | modern (recommended) or legacy               |
| BungeeCord, Waterfall | `config.yml`    | legacy (`ip_forward`), BungeeCord's only way |

| Game server                | Accepts the players of the proxy through                                                       |
| -------------------------- | ---------------------------------------------------------------------------------------------- |
| Paper, Purpur, Folia, Leaf | `config/paper-global.yml` (modern) or `spigot.yml` with `bungeecord: true` (legacy)            |
| Fabric, Quilt              | [FabricProxy-Lite](https://modrinth.com/mod/fabricproxy-lite) and the Fabric API (modern only) |
| Forge, NeoForge            | [Proxy-Compatible-Forge](https://modrinth.com/mod/proxy-compatible-forge) (modern or legacy)   |

Vanilla servers can't tell forwarded players apart and can't join. Game servers in a network run with
`online-mode=false`, as the proxy authenticates the players, and turn away anyone who doesn't come through the proxy.
The panel installs the forwarding mod of a Fabric, Quilt, Forge or NeoForge server from Modrinth when it joins, and
removes it when it leaves.

- **Map and routing.** The network's page shows where players connect, the proxy and its servers with their state and
  players. Players join the servers of the join order and fall back to the next one when a server is offline, full or
  kicks them (Velocity's `try`, BungeeCord's `priorities`); servers are dragged into another order. Host names, such as
  `survival.example.com`, send the players who connect through them to their own servers (`forced-hosts`, for
  BungeeCord one server each). Servers have the name players use with `/server`; BungeeCord's servers also have a MOTD
  for their host names and can be restricted to players with the permission `bungeecord.server.<name>`.
  Of more than 10 servers, the map shows those players join, fall back to or reach through a host name and those that
  crash, and folds away the others until it is expanded; the list of servers can be searched.
- **Changes.** All changes are saved and applied together, and the panel tells beforehand what they do. Servers that
  join or leave restart, all of them when the forwarding changes. The proxy reloads its configuration through its
  console (`velocity reload`, `greload`), which disconnects nobody; BungeeCord can't reload without a server it had, so
  removing or renaming a server restarts it. If a node is offline, the change is saved and **Apply again** configures
  its servers later. Proxies created by earlier versions are created again once, to read console commands.
- **Proxy configuration.** The proxy's tab of the network, and the **Configuration** tab of every proxy, edit the other
  settings of its file as a form: MOTD, the shown maximum of players, online mode, ping passthrough, compression,
  timeouts, rate limits, the HAProxy protocol, query, BungeeCord's permissions and more. Settings that aren't known
  appear under Advanced. Those the network decides, such as the servers, are locked. Saving reloads a running proxy.
- **Actions.** **Servers** starts all servers of the network before the proxy, so that players find them, and stops the
  proxy first, so that all players leave at once. **Message** sends a chat message to all running game servers.
- **Restart server by server.** The running game servers restart a few at a time (1, 2, 5 or 10) while the network
  stays open: the proxy first sends their players to another running server with `send`, and the next servers restart
  once these run again. The servers players join first restart last and one at a time; the proxy keeps running, and a
  server that doesn't start again stops the restart.
- **Maintenance.** The network's overview turns maintenance on and off with the
  [Maintenance](https://modrinth.com/plugin/maintenance) plugin on the proxy: the server list shows the network in
  maintenance, and only the team may join. The first time, the panel installs the plugin from Modrinth and restarts the
  proxy to load it, which disconnects all players once. The team is edited in the panel (the plugin's `maintenance add`
  and `remove`), the texts in the plugin's `config.yml`, which the panel links to. The panel reads the state from the
  plugin's files, so it also shows maintenance turned on in the game.
- **Reaching the servers.** On its own node, the proxy reaches a server by container name over a Docker network that only
  the two of them share; such a server's port isn't published at all. A server on another node is reached at that
  node's host and the server's port, which must be open for the proxy's node. Docker's rules bypass firewalls such as
  ufw, so allow only the proxy's node in Docker's `DOCKER-USER` chain on the server's node, e.g. for port 25566 and the
  proxy's node 203.0.113.10: `iptables -I DOCKER-USER -p tcp -m conntrack --ctorigdstport 25566 --ctdir ORIGINAL ! -s 203.0.113.10 -j DROP`
  (and save it, e.g. with `netfilter-persistent save`). The panel shows this command with each such server.
- **Legacy forwarding** doesn't prove that players come through the proxy: anyone who reaches such a server can join as
  any player. A network with legacy forwarding and servers on other nodes therefore needs the confirmation that a
  firewall protects them, and no server of it moves to another node without.
- New Minecraft servers start with a whitelist: add players on the **Players** page, for one server or the network.

## Players

The **Players** page lists the players online on all game servers, with their server and network, to search and act on;
`Ctrl+K` finds them too. Its other tabs join the ban list, whitelist and operators of all servers or of a network, with
how many servers have each player, and the changes that wait for stopped servers.

- **Actions.** Kick, ban (with a reason), pardon, add to and remove from the whitelist, make operator and take it away,
  and turn the whitelist on or off. A change goes to the player's server, the network or all servers, as chosen; bans
  go to the network first. Kicks leave the server: Velocity sends kicked players to another server of the network,
  BungeeCord disconnects them. **Send to another server** moves a player within the network through the proxy (`send`).
- **Stopped servers** get a change once they run again, so that a ban also reaches the servers of a network that are
  stopped. The agent keeps the waiting changes in `noryx-pending-players.json` in the server's data.
- **How.** The agents run Minecraft's own commands (`minecraft:ban` and so on) through the server's console port, so the
  lists stay in the server's files, also where plugins replace the commands, and read the lists from those files.
  Names are checked before they become part of a command: 16 letters, digits and underscores, or Floodgate's dot before.
- **Permissions.** Acting on players needs the permission to manage players on each server; making operators also needs
  the permission to send console commands, as operators may run any command in the game.

## Usage

The panel shows what nodes and servers use, now and during the last week.

- **Now.** Every agent measures every 5 seconds: CPU and memory of the node and of each server (without the page
  cache, like `docker stats`), the network traffic of each server, the size of its data (every 5 minutes), the players
  online and the ticks per second of Paper, Purpur and Leaf servers. The number of players comes from the status request that
  the server list in the game sends too, which game servers and Velocity answer; BungeeCord and Waterfall are left out,
  as they log every such request. The names of all players and the ticks per second come through the server's console
  port (`list`, `tps`) over a connection that stays open, because servers log every new one. Server cards show CPU, memory and players; the **Usage** tab of a server and the node page
  show the rest.
- **History.** The master records the latest measurement of every agent each minute and keeps it for a week. Charts
  show the last 24 hours (averages of 5 minutes) or 7 days (averages of 30 minutes), with the most players of each
  step, and a table shows the same values. Gaps are times in which a server didn't run or its node couldn't be
  reached. The history of a server moves and goes away with it.

## Logs

The master keeps a log of what happens on it and on its agents, so that it's clear who did what and what went wrong.

- **Actions.** Every request of the panel that changes something, and every download of a file, folder, backup or
  export, is logged with the user, the IP address, the node and server it concerned (with their names at that time),
  the outcome and how long it took. Denied requests are logged as warnings, failed ones as warnings or, if the master
  or an agent failed, errors. Sign-ins, failed sign-ins, password changes, changes of two-factor authentication,
  sign-outs, enrollments, certificate renewals and what backup jobs and policies did on each server are logged too.
- **Agents.** An agent logs every call it receives with its origin (the master or its local CLI) and keeps its latest
  entries in memory. The master collects them over the mutually authenticated connection and continues where it left
  off, also after a restart of either. Calls that only read are logged at the debug level, downloads at the info level.
- **Logs page.** Lists the entries newest first, with new ones streaming in. Filters for the time, level, category,
  source, node and a text search are part of the address, so a view can be shared. An entry opens to show all its
  details and narrows the list to its user, server or category. Key figures and a chart show the warnings and errors
  of the last 24 hours; selecting an hour shows its entries. The entries are exported as CSV or JSON lines.
- **Everywhere else.** A bell in the sidebar counts the new warnings and errors, and new ones show up as notifications,
  except those of the user's own actions. Servers have an **Activity** tab and nodes an **Activity** section.
- **Command line.** `noryx-master logs` and the terminal's `logs` command show the log with the same filters, also as
  JSON lines and following new entries with `-f`. `noryx-agent logs [-f]` shows an agent's own log, also while the master
  is unreachable.
- **Console and files.** Master and agent log to stderr as text or, with `--log-format json`, as JSON; `--log-level`
  chooses the least important level (`info` by default) and `--log-file` also writes JSON lines to a file, which is
  rotated at 10 MB with 5 older files kept.

Entries are kept for 30 days unless the settings say otherwise, and at most the newest million.

## Settings

The **Settings** page configures the master, gives an overview of the agents and manages who may do what. Each tab
only shows to users with the permission for it.

- **General** shows the running master (version, uptime, addresses, CA fingerprint, certificates) and its settings. The
  address the panel listens at (it replaces `--http-addr`; empty uses the flag again) and its HTTPS apply when the
  master starts again, e.g. with **Restart master** or the next update; the page says so until then. Only administrators
  change them, as they can open the panel to other networks. The address is checked when saved, and if the master can't
  listen there when it starts, e.g. because another program took the port, the panel falls back to `--http-addr` and
  shows why, so a wrong address can't lock you out. For HTTPS, the panel serves a self-signed certificate for the IP
  addresses and names of the machine, the domain if one is set, and the host of the enrollment address; it shows its
  fingerprint to compare with the browser's warning. Or it gets one from Let's Encrypt for its domain, which has to
  point to the machine: Let's Encrypt checks it at port 443 of the panel or at port 80, where the master then sends
  browsers to HTTPS. Until Let's Encrypt issued one, and for other names such as an IP address, the panel serves the
  self-signed certificate, and the page shows why. With a certificate of the command line (`--tls-cert`), these settings
  don't apply. **Restart master**, for administrators, stops the master and lets systemd start it again (also reading
  `master.env` again), unless servers are moving to another node; Minecraft servers keep running. The other settings
  apply right away: the enrollment address join tokens contain (it replaces `--public-enroll-addr`; empty uses the flag
  again), how long join tokens are valid (5 minutes to a day, 1 hour by default), how long sign-ins to the panel last (1
  hour to a week, 12 hours by default), how long log entries are kept (1 day to a year, 30 days by default), the port
  range and memory reserve that new nodes get, and whether the master looks for updates. Administrators can also look
  for an update right away.
- **Agents** lists all nodes with their agent version, certificate and settings, which can be changed there too.
- **Terminal** runs the commands of `noryx-agent` (`status`, `server …`, `backup …`) on any node, and the master's own
  commands: `status`, `node list`, `node renew <node>` and `logs`. `help` lists them; output streams in as it happens, e.g.
  for `server logs <id>`, and Ctrl+C stops a command. A node's page opens its terminal directly. Besides the
  permission to use the terminal, every command needs its own, e.g. `server restart <id>` that to restart this server.
- **Users** invites users, chooses their groups, disables and deletes them, creates setup links and turns off two-factor
  authentication for users who lost their phone.
- **Groups** defines what their members may do.

## Users and permissions

Users get their permissions from groups; a user can be in several groups and has the permissions of all of them.

- **Fine-grained permissions.** There are permissions for every action, by area: nodes (see, change, renew
  certificates, remove, add), servers (see, create, start, stop, restart, change settings, delete), console (read, send
  commands), players (kick, ban, whitelist and make operators), files and configuration (browse and download, change
  files, `server.properties`, plugins and mods),
  backups (see and download, back up, restore, delete), the log, networks, templates, backup jobs, policies, the
  master's settings, the terminal, users and groups. Choosing a permission also chooses what it needs, e.g. seeing the servers
  one may restart.
- **Scopes.** The node and server permissions of a group apply to all servers, or only to chosen nodes (including
  servers created later) and single servers, e.g. a group that may restart the lobby and use its console. Lists only
  show the nodes and servers a user may see, and the log only the entries about them; entries about the master, users
  or groups need the permission for all servers. Other permissions, e.g. for networks or policies, apply everywhere,
  because they act on any server.
- **Administrators.** The built-in Administrators group has every permission, also those that later versions add.
  Only its members see and install updates; no permission allows that to other groups.
  `noryx-master user add` creates administrators, e.g. the first one or after losing access. Existing users became
  administrators with this version.
- **Invitations.** New users get a setup link (valid for three days, usable once) to choose their password; the same
  link resets a forgotten password. The token is in the link's fragment, which browsers don't send to servers, and the
  panel removes it from the address bar once it was read. Users change their own password on their account page (their
  name in the sidebar), which signs them out everywhere else.
- **Two-factor authentication.** It is off until users set it up on their account page: they scan a QR code with an
  authenticator app (TOTP, e.g. Google Authenticator, Aegis or a password manager), confirm with a code of it and their
  password, and get 10 recovery codes that each replace a code once. Then signing in asks for a code after the
  password, and other sessions end. A setup link then only sets the password; signing in still needs a code. Turning
  it off and new recovery codes need the password. Users who may manage a
  user turn it off for them, e.g. after they lost their phone and recovery codes, but not for themselves; without any
  administrator who can still sign in, `noryx-master user add` creates a new one.

## Security model

- **Own CA.** The master creates an Ed25519 certificate authority on first start. All master ↔ agent traffic is
  TLS 1.3 with mutual authentication. Identities are names, not IPs (`master.noryx.internal`,
  `<node-id>.node.noryx.internal`), so nodes can change their address without re-enrolling.
- **Short-lived certificates.** Master and node certificates are valid for 90 days and renewed automatically once
  a third of their lifetime is left. For a node, the agent creates the new key and only sends a signing request; it
  installs the signed certificate after checking it, without a restart. The panel can renew a node on demand.
  A node that stays offline until its certificate expires has to be enrolled again with a new join token.
- **Enrollment.** Adding a node creates a single-use join token (valid for one hour unless the settings say
  otherwise, stored only as a hash). It contains the master address, the node ID, the secret and the CA fingerprint.
  The agent creates its key pair locally, sends a CSR and pins the CA fingerprint, so the exchange can't be
  intercepted. Private keys never leave the node. The master checks the token before it signs the CSR, and a
  client gets 5 attempts, then one every 12 seconds.
- **Agents only obey the master.** The agent requires a client certificate with the master identity. Node
  certificates are server-only, so a compromised node can't command other nodes. Locally, the agent is controlled
  through a Unix socket (mode `0600` inside a `0700` data directory).
- **Panel.** Argon2id password hashes, session tokens stored as SHA-256 hashes, `__Host-` cookies
  (`HttpOnly`, `Secure`, `SameSite=Strict`), cross-origin request protection, a strict Content Security Policy and
  self-hosted fonts. Sign-in attempts are rate limited per client address (IPv6 per /64 network) and per username,
  changes that need the password per user. A username has a larger budget than a client, so that a single client
  can't keep a user out. Client addresses come from the `X-Forwarded-For` or `X-Real-IP` header only for the reverse
  proxies named with `--trusted-proxy`.
- **Two-factor authentication.** Codes of the app (RFC 6238) work only once, and from the fifth wrong code in a row on,
  codes aren't checked for a minute that doubles with every further wrong one, up to a day; parallel guesses count
  too. Recovery codes have 50 random bits and are stored as SHA-256 hashes. The secret of the app is stored in the
  master's database, which needs the same protection as the CA key next to it. The panel must be served over HTTPS (its
  settings, a reverse proxy or `--tls-cert`/`--tls-key`), otherwise browsers drop the secure session cookie
  (`localhost` is exempt). With a certificate of Let's Encrypt or `--tls-cert`, the master tells browsers to use HTTPS
  only (HSTS, one year), but not with a self-signed one, which would lock browsers out once it changes; behind a
  reverse proxy, set it there. The master may listen at ports below 1024 (`CAP_NET_BIND_SERVICE`), e.g. 443 and 80,
  and has no other privileges.
- **Networks.** Velocity's modern forwarding signs the forwarded player data with a random secret per network, which
  is stored in the master's database and on the network's servers; the API never returns it, and the file manager hides
  it in every file that holds it. Legacy forwarding (BungeeCord's) can be spoofed by anyone who reaches a server, so the
  servers of such a network are only reachable by the proxy on its node, and those on other nodes need the operator's
  confirmation that a firewall protects them. The forwarding mods come from Modrinth like other mods, checked against
  their SHA-512 hashes. Proxies read console commands from their standard input, which only the agent writes to through
  Docker; no RCON plugin is added.
- **Containers.** Containers run with `no-new-privileges`, memory and PID limits, and only the capabilities the images
  need to hand the data to the server's user: `CHOWN`, `SETUID` and `SETGID`, for proxies also `DAC_READ_SEARCH`.
  Servers of a node can't reach each other: they share a Docker network without communication between containers
  (`noryx-servers`), and a Velocity proxy shares another one only with its backends on the node.
- **Agent input.** Every request is validated by the agent. Server files are confined to the data directory
  (`os.Root`), and servers are only created after the operator accepts the Minecraft EULA. JVM options may only
  contain characters that the image's start script can't interpret as shell syntax, can't override the memory limit
  and can't run code: Java agents, commands on errors, and debugging or JMX ports are refused.
- **File manager.** The agent confines every path to the server's data directory, including through symbolic
  links, and new files belong to the server's user. Downloads are sent as attachments with a sandboxing CSP, so an
  uploaded HTML file can't run scripts in the panel. Secrets of the server stay on the node: files that only hold
  them can't be listed, read, written or moved, others show them as `<hidden>`, no file or folder with secrets can be
  moved where they would show, and archives leave them out. Only moving a server to another node copies them.
- **Plugins.** The master downloads only from Modrinth's CDN, up to 256 MB, and only uses a file whose SHA-512 hash
  matches the one Modrinth's API lists. A modpack is checked the same way, and each of its files against the SHA-512
  hash in the pack; packs with files elsewhere than on Modrinth's CDN or with paths that leave the server's folder are
  refused before a server is created, and the agent confines the files like those of the file manager. The version of
  a mod loader ends up in a variable of the server image, so the agent only accepts letters, digits, `.`, `_`, `+`
  and `-`. The agent decides the folder from the server type and only accepts plain
  `.jar` file names in it. Project icons are fetched by the master, so the browser never contacts Modrinth and the
  Content Security Policy stays unchanged.
- **Duplicates.** Copying never follows symbolic links, so a copy can't pull in files from outside the server's
  directory. A copied Velocity proxy loses its forwarding secret, a copied BungeeCord proxy stops forwarding and a
  copied game server stops trusting the proxy, so a copy can't impersonate a server of a network. A copied Fabric or Quilt
  server keeps FabricProxy-Lite, which turns players away until it is removed in the **Mods** tab.
- **Backups.** The agent keeps backups outside of the servers' folders, accessible to itself only (mode `0700`), so a
  compromised server can't read or tamper with them. The master can only choose among the storage locations the
  node's administrator allowed, and backup IDs and paths are validated by the agent. Restoring confines every entry
  to the server's folder, and backups never contain symbolic links. Downloads are attachments like those of the file
  manager and hide the secrets the same way; the backups on the node keep them, so restoring works.
- **Permissions.** Every API route states the permission it needs when it is registered, so none can be added
  without; the terminal checks each command the same way and refuses commands without a check. Permissions are loaded
  for every request, so changes, disabling and deleting apply right away; disabled users are signed out. Streams that
  follow output, the console and terminal commands such as `server logs`, end every 5 minutes, so the panel checks
  the session and the permissions again; the console connects again on its own and continues. Users can only
  grant permissions they have themselves, within their own scope, and only manage users who have no more permissions
  than they do, so no one can raise their own permissions. The last enabled administrator can't be disabled, deleted or
  removed from the Administrators. The master logs every change with the user who made it, also denied attempts.
- **Terminal.** The panel's terminal is not a shell. The agent's commands are the same code as its local CLI, but
  run in the master and reach the agent through the existing mutually authenticated connection, so the agent offers
  nothing new to the master. Commands that only the node's administrator may run (`storage`, `enroll`) don't exist
  there. Command lines are split like a shell would, but nothing is expanded or executed by one, and the master logs
  every command with the user who ran it.
- **Log.** An agent can only add entries about its own node and its servers, at a limited rate, and entries are cut to
  a maximum size, so a compromised agent can't fill the database or write entries about other nodes. Request fields
  that may hold secrets, such as the forwarding secret of a network, are never logged. Exports protect spreadsheets
  from formulas in entries, and log files are only readable by their owner. A live stream ends every 5 minutes and the
  browser connects again, which checks the session and the permissions again. Behind a reverse proxy, the logged IP
  address is that of the proxy, unless `--trusted-proxy` names it.
- **Moving servers.** Agents never connect to each other: the master relays the server's archive and backups between
  them over its mutually authenticated connections. The new node checks the settings like those of a new server and
  extracts the archive confined to the server's data directory, without symbolic links. Moving needs the permissions
  to delete the server and read its files, and to create servers on the new node.
- **Usage.** To ask a server for its ticks per second, the agent reads the console password from the server's
  `server.properties` and connects to the server's console port inside Docker's network; the password never leaves
  the node.
- **Storage locations.** Only the node's administrator decides where server data may be stored
  (`noryx-agent storage add`). The panel can only choose among these locations, so a compromised master can't
  mount other host directories into containers.
- **Installation.** The installer only runs once it is downloaded completely, downloads only over HTTPS from the
  releases and installs a package only if its SHA-256 checksum matches the release's `checksums.txt`, and only if the
  release key that is part of the installer verifies its signature. The private key is a secret of the repository that
  only the job signing the release gets, so someone who can merely change releases, e.g. with a stolen token, can't make
  installations accept other files. Release files also carry build provenance attestations, signed through GitHub, that
  tie them to the release workflow and the tagged commit. The master runs as its own system user with a hardened systemd
  unit, and refuses a data directory of another user, so a command run as root can't leave files there that lock the
  master out.
- **Updates.** The master can't install anything itself. To update, it only creates a file that makes systemd start
  `noryx-master-update`, which runs as root but takes no input from the master: it installs the latest release with the
  installer of the installed package, checking the signature and the checksums. Agents only install releases newer than
  themselves, so even a compromised master can't downgrade a node to a vulnerable version, and the version must have the
  form of a release. `systemctl mask noryx-master-update.path` forbids updates from the panel.

## Repository layout

The code is organised by feature, not by layer.

```
api/noryx/v1/            gRPC contract (enrollment, node, server, files, properties, proxy, players, plugins, backups, log, stats, progress) and generated code
cmd/noryx-master/        master binary
cmd/noryx-agent/         agent binary
internal/pki/           CA, certificate issuing, mTLS configurations (shared)
internal/enrollment/    join token format (shared)
internal/agentcli/      commands that control a running agent, for its local CLI and the panel's terminal (shared)
internal/logging/       log setup: console, rotating file, attributes with a meaning, notes of requests (shared)
internal/master/
  app/                  wiring, HTTP and gRPC listeners, CLI
  auth/                 accounts, passwords, setup links, two-factor authentication, sessions, sign-in
  access/               permissions, groups with scopes, user management, the permission every API route needs
  settings/             settings of the master that the panel changes, and a description of the running master
  terminal/             runs the commands of the master and of the agents for the panel
  node/                 node registry, enrollment, agent connections
  update/               looks for new releases, updates the master through systemd and the agents after it
  server/               server API, forwarded to the node's agent
  tag/                  tags of servers, which the panel finds and groups them by
  operation/            long actions run in the background, with their steps and progress, and the API that follows them
  network/              networks of servers behind a proxy, applied through the agents; actions on their servers,
                        rolling restarts, maintenance and the settings of proxies
  player/               kicks, bans, whitelists and operators on many servers at once, their joined lists, and sending
                        players to another server of a network
  files/                file manager, streamed between the browser and the agent
  properties/           server.properties editor
  plugin/               installs, lists and removes plugins and mods of servers
  modrinth/             client for the Modrinth API and CDN
  modpack/              creates servers from Modrinth modpacks: checks a pack and writes its files into the server
  template/             templates for new servers
  schedule/             tasks that run on servers or nodes at set times: storage, scheduler, REST API
  backup/               backups of servers, and backup jobs as scheduled tasks
  policy/               policies as scheduled tasks: restarts with warnings, stops, starts, console commands
  database/             SQLite and embedded migrations
  logs/                 log in the database, logging of API requests, collecting the agents' logs, REST API, CLI
  usage/                history of what nodes and servers use, from the agents' measurements, REST API
  httpapi/              JSON helpers
internal/agent/
  app/                  wiring, listeners, local CLI
  enroll/               enrollment client
  node/                 machine info, certificate renewal, updates of the node
  server/               server lifecycle and input validation
  storage/              storage locations allowed for server data
  datadir/              confined access to a server's data, owned by the server's user
  network/              configuration of proxies and game servers for networks, the settings of proxies, and the
                        Maintenance plugin
  player/               kicks, bans, whitelists and operators with Minecraft's commands; changes for stopped servers
  files/                file access for the file manager
  properties/           reads and updates server.properties, keeping comments
  plugin/               plugin and mod files of servers
  backup/               backups of servers: selection, archives, restoring
  logs/                 latest log entries in memory, log service, logging of every call
  progress/             tells the master the progress of calls, e.g. downloading an image, through ProgressService
  stats/                measures what the node and its servers use: CPU, memory, network, data, players with names, TPS
  runtime/              runtime interface; docker/ implements it
internal/e2e/           end-to-end tests over real mTLS, with a fake runtime and a fake Modrinth
web/                    admin panel (React, Vite, Tailwind CSS, shadcn/ui)
  src/features/         auth, dashboard, nodes, servers, files, properties, networks, plugins, templates, backups,
                        policies, schedules (shared by backups and policies), settings, terminal, logs, usage,
                        access (users, groups and the permission checks of the panel), updates, palette (Ctrl+K),
                        operations (progress, notifications and the list of operations), players, modpacks
packaging/              installer, systemd units, options and package scripts; .goreleaser.yaml builds releases
```

## Development

`scripts/ubuntu-test.sh` sets up a complete test environment on Ubuntu (desktop, server or WSL): it installs
Go, Node.js and Docker when missing, builds everything, starts master and agent and can create a Paper test server.

Requirements: Go 1.27, Node.js 22. Nodes need Docker.

```sh
go run ./cmd/noryx-master --data-dir .data/master user add admin   # once, prompts for a password
make dev-master                                                    # API on :8080, enrollment on :9443
make dev-web                                                       # panel on http://localhost:5173
```

In the panel, add a node with the agent address `127.0.0.1:7443` and enroll the agent with its join token, which the
panel shows under "Installed the agent another way?":

```sh
go run ./cmd/noryx-agent --data-dir .data/agent enroll <join-token>
go run ./cmd/noryx-agent --data-dir .data/agent serve --listen 127.0.0.1:7443
```

| Command         | Purpose                                                                        |
| --------------- | ------------------------------------------------------------------------------ |
| `make build`    | Builds the panel and both binaries into `bin/`                                 |
| `make test`     | Runs all Go tests, including the end-to-end test                               |
| `make lint`     | golangci-lint, oxlint, the translation checks and the TypeScript type check    |
| `make generate` | Regenerates the gRPC code after changing `api/**/*.proto`                      |
| `make packages` | Builds the packages and archives of a release into `dist/`, without publishing |

**Translations.** The panel's texts are English and serve as the keys of their translations
([i18next](https://www.i18next.com)): components show them with `t("…")`, or `<Trans>` for texts with markup, and
`msg("…")` marks those outside of components. `npm run i18n` in `web` lists them in `web/src/locales/en.json` and adds
new ones to the other languages, e.g. `de.json`, untranslated: they are empty there and show in English until someone
translates them. `make lint` fails while the files are out of date or a component shows a text without `t`. A new
language is a copy of `en.json` with the texts translated and its code added to `locales` in `web/i18next.config.ts`;
the language menu offers it then.

**Releasing.** The release workflow tests, builds the panel and both programs with [GoReleaser](https://goreleaser.com)
and creates a draft of a GitHub release with the packages, archives, checksums, attestations and the installer, which
installs the version it belongs to. A second job signs `checksums.txt` with the secret `RELEASE_SIGNING_KEY` and
publishes the release. Start the workflow under **Actions → Release → Run workflow** with a version `vX.Y.Z`, or
`vX.Y.Z-rc.N` for a pre-release: it tags the newest commit of `main`. Pushing such a tag runs it too:

```sh
git tag v1.2.0 && git push origin v1.2.0
```

**Release key.** Installers only accept releases signed with the Ed25519 key in `RELEASE_KEY` of `packaging/install.sh`,
and the second job fails if the secret doesn't belong to it. A new key, e.g. for a fork, is created with
`openssl genpkey -algorithm ed25519 -out release.pem`, and `openssl pkey -in release.pem -pubout -outform DER | base64`
prints its value for `RELEASE_KEY`. Installations only learn of a new key through an update, so the first release that
has it in `RELEASE_KEY` is still signed with the old one.

## Roadmap

- More runtimes (plain processes)
