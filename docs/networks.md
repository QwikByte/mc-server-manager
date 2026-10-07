# Networks

A network puts game servers behind a proxy, on one node or spread across nodes. The panel doesn't add a network system
of its own: it writes the configuration the proxies and game servers document, and shows it as a picture. The master
stores the network and configures each server through its agent.

## Proxies and forwarding

| Proxy                 | Configuration   | Forwarding                                   |
| --------------------- | --------------- | -------------------------------------------- |
| Velocity              | `velocity.toml` | modern (recommended) or legacy               |
| BungeeCord, Waterfall | `config.yml`    | legacy (`ip_forward`), BungeeCord's only way |

Waterfall reached its [end of life](https://forums.papermc.io/threads/1088/) and can no longer download its command
modules, whose API PaperMC shut down. A new Waterfall proxy has no `send`, `/server`, `/glist`, `/alert` and `/find`, so
the panel no longer offers Waterfall for new servers and templates. Existing proxies keep running, and those that
downloaded their modules before keep them; their pages recommend Velocity with modern forwarding, or BungeeCord, which
reads the same `config.yml`, and a Waterfall network can [change its proxy](#changing-the-proxy) to one of them.

| Game server                | Accepts the players of the proxy through                                                       |
| -------------------------- | ---------------------------------------------------------------------------------------------- |
| Paper, Purpur, Folia, Leaf | `config/paper-global.yml` (modern) or `spigot.yml` with `bungeecord: true` (legacy)            |
| Fabric, Quilt              | [FabricProxy-Lite](https://modrinth.com/mod/fabricproxy-lite) and the Fabric API (modern only) |
| Forge, NeoForge            | [Proxy-Compatible-Forge](https://modrinth.com/mod/proxy-compatible-forge) (modern or legacy)   |

Vanilla servers can't tell forwarded players apart and can't join. Game servers in a network run with
`online-mode=false`, as the proxy authenticates the players, and turn away anyone who doesn't come through the proxy. A
server that leaves its network, and a copy of one, get `online-mode=true` again. The overview warns about a server
outside of networks that runs in offline mode, as anyone who reaches it can join under any name. The panel installs the
forwarding mod of a Fabric, Quilt, Forge or NeoForge server from Modrinth when it joins, and removes it when it leaves.

New Minecraft servers start with a whitelist: add players on the [Players](players.md) page, for one server or the
network.

## Map and routing

The network's page shows where players connect, the proxy and its servers with their state and players. Players join the
servers of the join order and fall back to the next one when a server is offline, full or kicks them (Velocity's `try`,
BungeeCord's `priorities`); servers are dragged into another order. Host names, such as `survival.example.com`, send the
players who connect through them to their own servers (`forced-hosts`, for BungeeCord one server each). Servers have the
name players use with `/server`; BungeeCord's servers also have a MOTD for their host names and can be restricted to
players with the permission `bungeecord.server.<name>`. Of more than 10 servers, the map shows those players join, fall
back to or reach through a host name and those that crash, and folds away the others until it is expanded; the list of
servers can be searched. **Players online** lists the network's players on the [Players](players.md) page.

## Applying changes

All changes are saved and applied together, and the panel tells beforehand what they do. Servers that join or leave
restart, all of them when the forwarding changes. The proxy reloads its configuration through its console
(`velocity reload`, `greload`), which disconnects nobody; BungeeCord can't reload without a server it had, so removing
or renaming a server restarts it. If a node is offline, the change is saved and **Apply again** configures its servers
later; until then, the network's page and the overview tell that its proxy may be out of date. The proxy reaches servers
on other nodes at the host of their node's address and their port, so changing either configures the network again.
Proxies created by earlier versions are created again once, to read console commands.

## Changing the proxy

A network can swap its proxy for a free Velocity or BungeeCord proxy on any node, and keeps its servers, join order,
host names and Bedrock port. The new proxy takes over the old one's configuration if it reads the same file, as
BungeeCord reads Waterfall's `config.yml`, and the Maintenance plugin with its state and team; other plugins stay with
the old proxy. Changing to Velocity can switch to modern forwarding, which restarts the servers once, as does a proxy
that comes to or leaves the node of a server. The dialog tells all this beforehand and where players join from then on.
The old proxy leaves the network and stops; the new one starts if either ran.

## Proxy configuration

The proxy's tab of the network, and the **Configuration** tab of every proxy, edit the other settings of its file as a
form: MOTD, the shown maximum of players, online mode, ping passthrough, compression, timeouts, rate limits, the HAProxy
protocol, query, BungeeCord's permissions and more. Settings that aren't known appear under Advanced. Those the network
decides, such as the servers, are locked. Saving reloads a running proxy.

## Actions

**Servers** starts all servers of the network before the proxy, so that players find them, and stops the proxy first, so
that all players leave at once. **Message** sends a chat message to all running game servers.

## Restart server by server

The running game servers restart a few at a time (1, 2, 5 or 10) while the network stays open: the proxy first sends
their players to another running server with `send`, and the next servers restart once these run again. The servers
players join first restart last and one at a time; the proxy keeps running, and a server that doesn't start again stops
the restart. It also stops before servers whose players the proxy can't send, e.g. a BungeeCord or Waterfall proxy
without its module `cmd_send`.

## Maintenance

The network's overview turns maintenance on and off with the [Maintenance](https://modrinth.com/plugin/maintenance)
plugin on the proxy: the server list shows the network in maintenance, and only the team may join. The first time, the
panel installs the plugin from Modrinth and restarts the proxy to load it, which disconnects all players once. The team
is edited in the panel (the plugin's `maintenance add` and `remove`), the texts in the plugin's `config.yml`, which the
panel links to. The panel reads the state from the plugin's files, so it also shows maintenance turned on in the game.

## Bedrock players

The network's overview lets players of the Bedrock Edition join, on phones, consoles and Windows, at a UDP port of the
proxy's node: the panel installs [Geyser](https://geysermc.org), from Modrinth, and Floodgate, from GeyserMC's download
server, on the proxy, publishes the port and writes it with `auth-type: floodgate` into Geyser's `config.yml`. Geyser
translates their game, and Floodgate lets them join without a Java account, which needs no plugin on the game servers:
Floodgate's key stays on the proxy. Letting Bedrock players in, a new port and **Apply again** update both to their
newest build; other changes of the network leave them be, so that they don't disconnect players. The proxy restarts when
Bedrock players are let in or no longer, their port changes or a plugin is updated; the game servers restart when they
are let in or no longer, as they stop demanding signed chat messages, which Bedrock players can't send
(`ENFORCE_SECURE_PROFILE=FALSE`, locked in their properties; afterwards `enforce-secure-profile=true` again). Java
players who only show secure chat don't see what Bedrock players write. The panel shows where Bedrock players connect
and warns about what keeps them out: Geyser joins as one Minecraft version (it tells which), so game servers of older
versions need ViaVersion and newer ones ViaVersion and ViaBackwards; Geyser needs about 1 GB of memory on the proxy; and
the UDP port must be open on the proxy's node, e.g. in the provider's firewall. Geyser also needs to reach Mojang's and
Microsoft's servers from the proxy, and Bedrock players can't join servers whose mods players must install. In the panel
and the game, Floodgate starts their names with a dot, e.g. `.Steve`. Turning Bedrock off removes both plugins, but
keeps their settings and Floodgate's key for later.

## Reaching the servers

On its own node, the proxy reaches a server by container name over a Docker network that only the two of them share;
such a server's port isn't published at all. A server on another node is reached over the private network of the nodes
if both nodes are part of it (see [Private network](#private-network)). Otherwise it is reached at that node's host and
the server's port, which must be open for the proxy's node. Docker's rules bypass firewalls such as ufw, so allow only
the proxy's node in Docker's `DOCKER-USER` chain on the server's node, e.g. for port 25566 and the proxy's node
203.0.113.10:
`iptables -I DOCKER-USER -p tcp -m conntrack --ctorigdstport 25566 --ctdir ORIGINAL ! -s 203.0.113.10 -j DROP` (and save
it, e.g. with `netfilter-persistent save`). The panel shows this command with each such server; with Docker's nftables
firewall there is no `DOCKER-USER` chain, so use the private network instead.

## Private network

Nodes form a private WireGuard network, `10.213.0.0/24` with UDP port 51820 unless its settings on the **Nodes** page
say otherwise. Its range changes only while no node is part of it. A node joins on its page once its administrator ran
`noryx-agent overlay allow` on it, and gets the lowest free address; the others reach it at the host of its agent's
address, or at an endpoint set on its page, which a node on the master's machine needs (`install.sh all` registers it at
`127.0.0.1`). The proxy then reaches a server of another member at that node's address in the network, where the node
publishes the server's port only for the proxy's node: no firewall rule and no confirmation for legacy forwarding are
needed, and the traffic between the nodes is encrypted. Joining restarts nothing: a network shows which of its servers
move to the private network when it is applied again, which restarts them once for their new port. Leaving applies the
networks of the node again, which reach the servers at public ports then, and is refused while a network with legacy
forwarding would reach servers that no firewall protects. Nodes outside, agents of older versions and kernels without
WireGuard (it is part of Linux since 5.6; RHEL 9 has it only as an unsupported Technology Preview) work as before. The
master configures the members every 5 minutes, so those that were offline catch up, and right away after a change, e.g.
when a node is removed or its address changes. Each node's page shows its address, endpoint, key and peers with their
latest handshake and traffic, and rotates its key; the overview names members that had no handshake with another one for
5 minutes, e.g. as the UDP port is closed. The kernel keeps the interface `noryx0` while the agent restarts or updates,
and `noryx-overlay.service` restores it at boot, before Docker starts the servers.

## Legacy forwarding

Legacy forwarding doesn't prove that players come through the proxy: anyone who reaches such a server can join as any
player. A network with legacy forwarding and servers on other nodes outside the private network therefore needs the
confirmation that a firewall protects them, and no server of it moves to such a node without.
