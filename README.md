# Noryx

Manage Minecraft servers and whole networks from one admin panel. A **master** serves the panel and controls **agents**
on any number of dedicated servers, which run the Minecraft servers in Docker.

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

## Features

- **Servers:** Vanilla, Paper, Purpur, Folia, Leaf, Fabric, Quilt, Forge and NeoForge, and Velocity and BungeeCord
  proxies, with a live console, a file manager, a `server.properties` editor, copies and moves between nodes.
- **Networks:** game servers behind a proxy across nodes, with routing, restarts server by server, maintenance, Bedrock
  players and a private WireGuard network of the nodes.
- **Databases:** MariaDB and PostgreSQL for the plugins of a network.
- **Library:** plugins and mods from Modrinth and Hangar with their updates, servers from Modrinth modpacks, templates,
  and file sets that share configuration files between servers.
- **Automation:** backup jobs, restarts that warn the players, and starts, stops and console commands at set times.
- **Players:** kicks, bans, whitelists and operators across servers and networks, Bedrock players included.
- **Teams:** users and groups with fine-grained permissions, two-factor authentication and a log of who did what.
- **Security:** mutual TLS with its own CA, agents that only obey the master, hardened containers and signed releases.

The panel speaks English and German. Servers run in Docker; more runtimes, such as plain processes, are planned.

## Installation

Noryx runs on current Linux systems with systemd and OpenSSL 3, e.g. Debian 12, Ubuntu 22.04 or RHEL 9 and newer, on
x86_64 and arm64. Install the master, which serves the panel:

```sh
curl -fsSLO https://github.com/QwikByte/noryx/releases/latest/download/install.sh && sudo bash install.sh master
```

The panel listens at `127.0.0.1:8080` unless you choose another address. Open it through an SSH tunnel
(`ssh -L 8080:127.0.0.1:8080 <user>@<master>`, then `http://localhost:8080`) and turn on
[HTTPS](docs/installation.md#https) before you use it from elsewhere. Then add nodes under **Nodes**, which shows the
command that installs the agent on a node. Open port 9443 of the master for the nodes, and port 7443 of each node for
the master only. `sudo bash install.sh all` puts master and agent on one machine.

The master looks for new releases and offers them to administrators in the panel, which installs them on the master and
all nodes while the Minecraft servers keep running.

## Documentation

| Page                                                      | Covers                                                                         |
| --------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [Installation and operation](docs/installation.md)        | Installing, HTTPS, updates, files, commands, removing, verifying releases      |
| [The panel](docs/panel.md)                                | Overview, navigation, search, operations, languages                            |
| [Servers](docs/servers.md)                                | Console, files, `server.properties`, settings, crashes, copies, moving, tags   |
| [Networks](docs/networks.md)                              | Proxies, forwarding, routing, maintenance, Bedrock players, private network    |
| [Databases](docs/databases.md)                            | MariaDB and PostgreSQL datastores of networks                                  |
| [Templates, file sets, plugins and mods](docs/library.md) | Templates, file sets, Modrinth, Hangar, modpacks                               |
| [Backups and schedules](docs/automation.md)               | Backups of servers, datastores and the master; scheduled restarts and commands |
| [Players](docs/players.md)                                | Kicks, bans, whitelists and operators                                          |
| [Usage and logs](docs/monitoring.md)                      | Measurements, their history and the log                                        |
| [Settings, users and permissions](docs/administration.md) | Settings of the master, terminal, users, groups, two-factor authentication     |
| [Security model](docs/security.md)                        | How master, agents and servers are protected                                   |
| [Development](docs/development.md)                        | Building, running locally, translations, releasing                             |

## Development

Go 1.27, Node.js 22 and Docker: `make build` builds the panel and both programs, `make test` and `make lint` check them.
[docs/development.md](docs/development.md) explains the rest, and AI agents start with [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE)
