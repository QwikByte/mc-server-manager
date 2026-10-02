# MC Server Manager

All-in-one management for Minecraft servers and whole networks. One **master** with an admin panel
controls **agents** on any number of dedicated servers.

```
                                   ┌──────────────── node: dedicated server ───────────────┐
 Browser ──HTTPS──▶ mcsm-master ───┼─gRPC + mTLS──▶ mcsm-agent ──▶ Docker: Paper, Velocity…│
 (admin panel)      panel, REST API│                  ▲                                     │
                    enrollment     │  local CLI ──Unix socket                               │
                    SQLite, CA     └────────────────────────────────────────────────────────┘
```

| Program       | Runs on              | Responsibility                                                                                     |
| ------------- | -------------------- | -------------------------------------------------------------------------------------------------- |
| `mcsm-master` | the panel host       | Admin panel (embedded), REST API, node registry, certificate authority, enrollment endpoint        |
| `mcsm-agent`  | every dedicated host | Runs the Minecraft servers through a runtime (Docker), accepts commands from the master or its CLI |

Servers run as containers based on [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server)
(Vanilla, Paper, Purpur, Fabric, Forge, NeoForge) and [itzg/mc-proxy](https://github.com/itzg/docker-mc-proxy)
(Velocity, BungeeCord). Container labels are the agent's only state, so servers keep running while an agent restarts.

Each server has a live console in the panel: its output streams in as it happens, and commands go to game servers
through the RCON connection the server image provides.

The file manager of a server browses its data, uploads files by drag and drop (up to 16 GB each, streamed through
the master), edits configuration files in the browser and downloads files or whole folders as ZIP archives.

`server.properties` can be edited as a form: grouped settings with switches, choices and validated numbers, a
MOTD editor with colour codes and preview, and a search. Only properties of the server's Minecraft version are
shown, comments in the file are kept, and properties the manager relies on (container port, RCON) are locked.
Secrets such as the RCON password never reach the panel.

The settings of a server can be changed after it was created: name, Minecraft version, memory, port, Java
version (8, 11, 17, 21, 25 or the newest), when it starts on its own, Aikar's flags, JVM options and a CPU limit.
The agent creates the container again with the same data; the old container is only removed once the new one
exists.

A server can be duplicated on its node: the copy gets all files, worlds and settings under a new name and port, and
starts stopped. A running game server first writes its worlds to disk and pauses saving while they are copied, so
players stay connected. The copy doesn't take over the original's place in a network.

Each node has settings for its servers: the storage location preselected for new servers, a port range (new
servers get the first free port in it) and a memory limit, so that servers together can't get more memory than
the node has minus a reserve for the system (1 GB unless changed). Name and agent address can be changed too.

## Templates

A template preconfigures new servers: software, Minecraft version, memory, the settings above, `server.properties`
and a list of plugins or mods from Modrinth. When a server is created from a template, only the node, name, port and
storage are chosen; `server.properties` is written before the first start and each plugin is installed in the newest
release that suits the server, so templates don't go stale. Templates are created from scratch or from an existing
server ("Save as template"), which takes its settings, its properties (except those the manager sets) and the plugins
that come from Modrinth. Worlds and plugin configurations are not part of templates.

## Plugins and mods

Plugins (Paper, Purpur, Velocity, BungeeCord) and mods (Fabric, Forge, NeoForge) are installed from
[Modrinth](https://modrinth.com), either on any number of servers at once from the **Plugins** page or from the
**Plugins**/**Mods** tab of a server. The master picks the newest release for each server's software and Minecraft
version, installs the projects it requires, and replaces an older version of the same project. Installed files are
recognised by their hash, so the tab shows their project, version and available updates, also for files uploaded
by hand. Own `.jar` files can be uploaded too. Servers load changes when they restart.

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
- Deleting a server deletes its backups too. Locally, `mcsm-agent backup list|create|restore` works without the
  master, e.g. to restore a server while the master is unreachable.

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

A network puts Paper or Purpur servers behind a Velocity proxy, on one node or spread across nodes. The master stores
the network and configures each server through its agent; changes are applied to all servers of the network.

- The proxy's `velocity.toml` lists the servers and enables modern player forwarding. Players join the first server
  and switch with `/server <name>`. Other settings are kept, but comments in the file are not.
- Backends turn on Velocity forwarding in `config/paper-global.yml` and run with `online-mode=false`, as the proxy
  authenticates the players. They turn away anyone who doesn't come through the proxy.
- On its own node, the proxy reaches a server by container name over the Docker network `mcsm`. A server on another
  node is reached at that node's host and the server's port, which must be open for the proxy's node.
- Servers only restart if their configuration changed. Changing the servers of a network restarts the proxy, which
  disconnects all players. If a node is offline, the change is saved and **Apply again** configures its servers later.
- Proxies created by earlier versions are recreated on their first network change, because they used the wrong data
  directory and port inside the container.
- New Minecraft servers start with a whitelist: add players with `whitelist add <name>` in each server's console.

## Logs

The master keeps a log of what happens on it and on its agents, so that it's clear who did what and what went wrong.

- **Actions.** Every request of the panel that changes something, and every download of a file, folder, backup or
  export, is logged with the user, the IP address, the node and server it concerned (with their names at that time),
  the outcome and how long it took. Denied requests are logged as warnings, failed ones as warnings or, if the master
  or an agent failed, errors. Sign-ins, failed sign-ins, password changes, sign-outs, enrollments, certificate renewals
  and what backup jobs and policies did on each server are logged too.
- **Agents.** An agent logs every call it receives with its origin (the master or its local CLI) and keeps its latest
  entries in memory. The master collects them over the mutually authenticated connection and continues where it left
  off, also after a restart of either. Calls that only read are logged at the debug level, downloads at the info level.
- **Logs page.** Lists the entries newest first, with new ones streaming in. Filters for the time, level, category,
  source, node and a text search are part of the address, so a view can be shared. An entry opens to show all its
  details and narrows the list to its user, server or category. Key figures and a chart show the warnings and errors
  of the last 24 hours; selecting an hour shows its entries. The entries are exported as CSV or JSON lines.
- **Everywhere else.** A bell in the sidebar counts the new warnings and errors, and new ones show up as notifications,
  except those of the user's own actions. Servers have an **Activity** tab and nodes an **Activity** section.
- **Command line.** `mcsm-master logs` and the terminal's `logs` command show the log with the same filters, also as
  JSON lines and following new entries with `-f`. `mcsm-agent logs [-f]` shows an agent's own log, also while the master
  is unreachable.
- **Console and files.** Master and agent log to stderr as text or, with `--log-format json`, as JSON; `--log-level`
  chooses the least important level (`info` by default) and `--log-file` also writes JSON lines to a file, which is
  rotated at 10 MB with 5 older files kept.

Entries are kept for 30 days unless the settings say otherwise, and at most the newest million.

## Settings

The **Settings** page configures the master, gives an overview of the agents and manages who may do what. Each tab
only shows to users with the permission for it.

- **General** shows the running master (version, uptime, addresses, CA fingerprint, certificate) and its settings,
  which apply right away: the enrollment address join tokens contain (it replaces `--public-enroll-addr`; empty uses the
  flag again), how long join tokens are valid (5 minutes to a day, 1 hour by default), how long sign-ins to the panel
  last (1 hour to a week, 12 hours by default), how long log entries are kept (1 day to a year, 30 days by default) and
  the port range and memory reserve that new nodes get.
- **Agents** lists all nodes with their agent version, certificate and settings, which can be changed there too.
- **Terminal** runs the commands of `mcsm-agent` (`status`, `server …`, `backup …`) on any node, and the master's own
  commands: `status`, `node list`, `node renew <node>` and `logs`. `help` lists them; output streams in as it happens, e.g.
  for `server logs <id>`, and Ctrl+C stops a command. A node's page opens its terminal directly. Besides the
  permission to use the terminal, every command needs its own, e.g. `server restart <id>` that to restart this server.
- **Users** invites users, chooses their groups, disables and deletes them, and creates setup links.
- **Groups** defines what their members may do.

## Users and permissions

Users get their permissions from groups; a user can be in several groups and has the permissions of all of them.

- **Fine-grained permissions.** There are permissions for every action, by area: nodes (see, change, renew
  certificates, remove, add), servers (see, create, start, stop, restart, change settings, delete), console (read, send
  commands), files and configuration (browse and download, change files, `server.properties`, plugins and mods),
  backups (see and download, back up, restore, delete), the log, networks, templates, backup jobs, policies, the
  master's settings, the terminal, users and groups. Choosing a permission also chooses what it needs, e.g. seeing the servers
  one may restart.
- **Scopes.** The node and server permissions of a group apply to all servers, or only to chosen nodes (including
  servers created later) and single servers, e.g. a group that may restart the lobby and use its console. Lists only
  show the nodes and servers a user may see, and the log only the entries about them; entries about the master, users
  or groups need the permission for all servers. Other permissions, e.g. for networks or policies, apply everywhere,
  because they act on any server.
- **Administrators.** The built-in Administrators group has every permission, also those that later versions add.
  `mcsm-master user add` creates administrators, e.g. the first one or after losing access. Existing users became
  administrators with this version.
- **Invitations.** New users get a setup link (valid for three days, usable once) to choose their password; the same
  link resets a forgotten password. The token is in the link's fragment, which browsers don't send to servers, and the
  panel removes it from the address bar once it was read. Users change their own password in the panel, which signs
  them out everywhere else.

## Security model

- **Own CA.** The master creates an Ed25519 certificate authority on first start. All master ↔ agent traffic is
  TLS 1.3 with mutual authentication. Identities are names, not IPs (`master.mcsm.internal`,
  `<node-id>.node.mcsm.internal`), so nodes can change their address without re-enrolling.
- **Short-lived certificates.** Master and node certificates are valid for 90 days and renewed automatically once
  a third of their lifetime is left. For a node, the agent creates the new key and only sends a signing request; it
  installs the signed certificate after checking it, without a restart. The panel can renew a node on demand.
  A node that stays offline until its certificate expires has to be enrolled again with a new join token.
- **Enrollment.** Adding a node creates a single-use join token (valid for one hour unless the settings say
  otherwise, stored only as a hash). It contains the master address, the node ID, the secret and the CA fingerprint.
  The agent creates its key pair locally, sends a CSR and pins the CA fingerprint, so the exchange can't be
  intercepted. Private keys never leave the node.
- **Agents only obey the master.** The agent requires a client certificate with the master identity. Node
  certificates are server-only, so a compromised node can't command other nodes. Locally, the agent is controlled
  through a Unix socket (mode `0600` inside a `0700` data directory).
- **Panel.** Argon2id password hashes, session tokens stored as SHA-256 hashes, `__Host-` cookies
  (`HttpOnly`, `Secure`, `SameSite=Strict`), cross-origin request protection, sign-in rate limiting, a strict
  Content Security Policy and self-hosted fonts. The panel must be served over HTTPS (reverse proxy or
  `--tls-cert`/`--tls-key`), otherwise browsers drop the secure session cookie (`localhost` is exempt).
- **Networks.** Only Velocity's modern forwarding is supported: it signs the forwarded player data with a random
  secret per network. BungeeCord's forwarding can be spoofed by anyone who reaches a backend. The secret is stored in
  the master's database and on the network's servers; the API never returns it.
- **Agent input.** Every request is validated by the agent. Server files are confined to the data directory
  (`os.Root`), containers run with `no-new-privileges` and memory and PID limits, and servers are only created
  after the operator accepts the Minecraft EULA. JVM options may only contain characters that the image's start
  script can't interpret as shell syntax, and can't override the memory limit.
- **File manager.** The agent confines every path to the server's data directory, including through symbolic
  links, and new files belong to the server's user. Downloads are sent as attachments with a sandboxing CSP, so an
  uploaded HTML file can't run scripts in the panel.
- **Plugins.** The master downloads only from Modrinth's CDN, up to 256 MB, and only uses a file whose SHA-512 hash
  matches the one Modrinth's API lists. The agent decides the folder from the server type and only accepts plain
  `.jar` file names in it. Project icons are fetched by the master, so the browser never contacts Modrinth and the
  Content Security Policy stays unchanged.
- **Duplicates.** Copying never follows symbolic links, so a copy can't pull in files from outside the server's
  directory. A copied proxy loses its forwarding secret and a copied backend stops trusting the proxy, so a copy
  can't impersonate a server of a network.
- **Backups.** The agent keeps backups outside of the servers' folders, accessible to itself only (mode `0700`), so a
  compromised server can't read or tamper with them. The master can only choose among the storage locations the
  node's administrator allowed, and backup IDs and paths are validated by the agent. Restoring confines every entry
  to the server's folder, and backups never contain symbolic links. Downloads are attachments like those of the file
  manager.
- **Permissions.** Every API route states the permission it needs when it is registered, so none can be added
  without; the terminal checks each command the same way and refuses commands without a check. Permissions are loaded
  for every request, so changes, disabling and deleting apply right away; disabled users are signed out. Users can only
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
  address is that of the proxy.
- **Storage locations.** Only the node's administrator decides where server data may be stored
  (`mcsm-agent storage add`). The panel can only choose among these locations, so a compromised master can't
  mount other host directories into containers.

## Repository layout

The code is organised by feature, not by layer.

```
api/mcsm/v1/            gRPC contract (enrollment, node, server, files, properties, plugins, backups, log) and generated code
cmd/mcsm-master/        master binary
cmd/mcsm-agent/         agent binary
internal/pki/           CA, certificate issuing, mTLS configurations (shared)
internal/enrollment/    join token format (shared)
internal/agentcli/      commands that control a running agent, for its local CLI and the panel's terminal (shared)
internal/logging/       log setup: console, rotating file, attributes with a meaning, notes of requests (shared)
internal/master/
  app/                  wiring, HTTP and gRPC listeners, CLI
  auth/                 accounts, passwords, setup links, sessions, sign-in
  access/               permissions, groups with scopes, user management, the permission every API route needs
  settings/             settings of the master that the panel changes, and a description of the running master
  terminal/             runs the commands of the master and of the agents for the panel
  node/                 node registry, enrollment, agent connections
  server/               server API, forwarded to the node's agent
  network/              networks of servers behind a proxy, applied through the agents
  files/                file manager, streamed between the browser and the agent
  properties/           server.properties editor
  plugin/               installs, lists and removes plugins and mods of servers
  modrinth/             client for the Modrinth API and CDN
  template/             templates for new servers
  schedule/             tasks that run on servers or nodes at set times: storage, scheduler, REST API
  backup/               backups of servers, and backup jobs as scheduled tasks
  policy/               policies as scheduled tasks: restarts with warnings, stops, starts, console commands
  database/             SQLite and embedded migrations
  logs/                 log in the database, logging of API requests, collecting the agents' logs, REST API, CLI
  httpapi/              JSON helpers
internal/agent/
  app/                  wiring, listeners, local CLI
  enroll/               enrollment client
  node/                 machine info
  server/               server lifecycle and input validation
  storage/              storage locations allowed for server data
  datadir/              confined access to a server's data, owned by the server's user
  network/              proxy and backend configuration for networks
  files/                file access for the file manager
  properties/           reads and updates server.properties, keeping comments
  plugin/               plugin and mod files of servers
  backup/               backups of servers: selection, archives, restoring
  logs/                 latest log entries in memory, log service, logging of every call
  runtime/              runtime interface; docker/ implements it
internal/e2e/           end-to-end tests over real mTLS, with a fake runtime and a fake Modrinth
web/                    admin panel (React, Vite, Tailwind CSS, shadcn/ui)
  src/features/         auth, nodes, servers, files, properties, networks, plugins, templates, backups, policies,
                        schedules (shared by backups and policies), settings, terminal, logs,
                        access (users, groups and the permission checks of the panel)
deploy/systemd/         service units
```

## Development

`scripts/ubuntu-test.sh` sets up a complete test environment on Ubuntu (desktop, server or WSL): it installs
Go, Node.js and Docker when missing, builds everything, starts master and agent and can create a Paper test server.

Requirements: Go 1.27, Node.js 22. Nodes need Docker.

```sh
go run ./cmd/mcsm-master --data-dir .data/master user add admin   # once, prompts for a password
make dev-master                                                    # API on :8080, enrollment on :9443
make dev-web                                                       # panel on http://localhost:5173
```

In the panel, add a node with the agent address `127.0.0.1:7443` and run the commands it shows:

```sh
go run ./cmd/mcsm-agent --data-dir .data/agent enroll <join-token>
go run ./cmd/mcsm-agent --data-dir .data/agent serve --listen 127.0.0.1:7443
```

| Command         | Purpose                                                  |
| --------------- | -------------------------------------------------------- |
| `make build`    | Builds the panel and both binaries into `bin/`           |
| `make test`     | Runs all Go tests, including the end-to-end test         |
| `make lint`     | golangci-lint, oxlint and the TypeScript type check      |
| `make generate` | Regenerates the gRPC code after changing `api/**/*.proto` |

## Deployment

`make build` produces two static binaries; the master contains the panel. Example systemd units are in
[`deploy/systemd`](deploy/systemd).

**Master**

1. `useradd --system --no-create-home mcsm`, install `bin/mcsm-master` to `/usr/local/bin` and the unit file.
   Set `--public-enroll-addr` to the host and port agents use to reach the master (or later in the panel's settings).
2. `sudo -u mcsm mcsm-master --data-dir /var/lib/mcsm-master user add admin`
3. Put a TLS reverse proxy in front of `127.0.0.1:8080` and open port 9443 for the agents.
   For plugins and mods, the master needs HTTPS access to `api.modrinth.com` and `cdn.modrinth.com`.
4. systemd's journal keeps what the master logs to stderr; add `--log-format json` for log collectors. For a log
   file, add `LogsDirectory=mcsm-master` to the unit and `--log-file /var/log/mcsm-master/master.log` to the command.
   `sudo -u mcsm mcsm-master --data-dir /var/lib/mcsm-master logs` reads the log on the host.

**Agent** (on every node)

1. Install Docker, `bin/mcsm-agent` and the unit file.
2. Add the node in the panel and run `sudo mcsm-agent enroll <join-token>`.
3. Start the service and allow port 7443 only from the master's IP.
4. Check the node locally with `sudo mcsm-agent status`; `sudo mcsm-agent server logs <id>` follows a console and
   `sudo mcsm-agent logs -f` the agent's own log.
5. Optionally allow more directories for server data, e.g. on a faster disk:
   `sudo mcsm-agent storage add ssd /mnt/ssd/mcsm`. New servers can then be created there from the panel, and
   backup jobs can keep their backups there, e.g. on another disk than the servers.
6. `sudo mcsm-agent backup list <id>` lists the backups of a server, `backup create <id>` backs it up and
   `backup restore <id> <backup-id>` restores it, also while the master is unreachable.

## Roadmap

- Console commands for proxies, so that network changes reload the proxy instead of restarting it
- More runtimes (plain processes)
- Two-factor authentication
- German translation of the panel
