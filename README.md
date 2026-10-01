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

## Security model

- **Own CA.** The master creates an Ed25519 certificate authority on first start. All master ↔ agent traffic is
  TLS 1.3 with mutual authentication. Identities are names, not IPs (`master.mcsm.internal`,
  `<node-id>.node.mcsm.internal`), so nodes can change their address without re-enrolling.
- **Short-lived certificates.** Master and node certificates are valid for 90 days and renewed automatically once
  a third of their lifetime is left. For a node, the agent creates the new key and only sends a signing request; it
  installs the signed certificate after checking it, without a restart. The panel can renew a node on demand.
  A node that stays offline until its certificate expires has to be enrolled again with a new join token.
- **Enrollment.** Adding a node creates a single-use join token (valid for one hour, stored only as a hash). It
  contains the master address, the node ID, the secret and the CA fingerprint. The agent creates its key pair
  locally, sends a CSR and pins the CA fingerprint, so the exchange can't be intercepted. Private keys never leave
  the node.
- **Agents only obey the master.** The agent requires a client certificate with the master identity. Node
  certificates are server-only, so a compromised node can't command other nodes. Locally, the agent is controlled
  through a Unix socket (mode `0600` inside a `0700` data directory).
- **Panel.** Argon2id password hashes, session tokens stored as SHA-256 hashes, `__Host-` cookies
  (`HttpOnly`, `Secure`, `SameSite=Strict`), cross-origin request protection, sign-in rate limiting, a strict
  Content Security Policy and self-hosted fonts. The panel must be served over HTTPS (reverse proxy or
  `--tls-cert`/`--tls-key`), otherwise browsers drop the secure session cookie (`localhost` is exempt).
- **Agent input.** Every request is validated by the agent. Server files are confined to the data directory
  (`os.Root`), containers run with `no-new-privileges` and memory and PID limits, and servers are only created
  after the operator accepts the Minecraft EULA.

## Repository layout

The code is organised by feature, not by layer.

```
api/mcsm/v1/            gRPC contract (enrollment, node, server) and generated code
cmd/mcsm-master/        master binary
cmd/mcsm-agent/         agent binary
internal/pki/           CA, certificate issuing, mTLS configurations (shared)
internal/enrollment/    join token format (shared)
internal/master/
  app/                  wiring, HTTP and gRPC listeners, CLI
  auth/                 administrators, sessions, sign-in
  node/                 node registry, enrollment, agent connections
  server/               server API, forwarded to the node's agent
  database/             SQLite and embedded migrations
  httpapi/              JSON helpers
internal/agent/
  app/                  wiring, listeners, local CLI
  enroll/               enrollment client
  node/                 machine info
  server/               server lifecycle and input validation
  runtime/              runtime interface; docker/ implements it
internal/e2e/           end-to-end test: enrollment and control over real mTLS
web/                    admin panel (React, Vite, Tailwind CSS, shadcn/ui)
  src/features/         auth, nodes, servers
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
   Set `--public-enroll-addr` to the host and port agents use to reach the master.
2. `sudo -u mcsm mcsm-master --data-dir /var/lib/mcsm-master user add admin`
3. Put a TLS reverse proxy in front of `127.0.0.1:8080` and open port 9443 for the agents.

**Agent** (on every node)

1. Install Docker, `bin/mcsm-agent` and the unit file.
2. Add the node in the panel and run `sudo mcsm-agent enroll <join-token>`.
3. Start the service and allow port 7443 only from the master's IP.
4. Check the node locally with `sudo mcsm-agent status`; `sudo mcsm-agent server logs <id>` follows a console.

## Roadmap

- Console commands for proxies
- File manager and backups
- Networks: register backend servers with their Velocity/BungeeCord proxy automatically
- More runtimes (plain processes) and Java version selection per server
- Roles, two-factor authentication and an audit log
- German translation of the panel
