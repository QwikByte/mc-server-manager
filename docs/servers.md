# Servers

Everything a single server offers in the panel. New servers can also start from a [template](library.md#templates) or a
[Modrinth modpack](library.md#modpacks).

## Software

Servers run as containers based on [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server) (Vanilla,
Paper, Purpur, Folia, Leaf, Fabric, Quilt, Forge, NeoForge) and [itzg/mc-proxy](https://github.com/itzg/docker-mc-proxy)
(Velocity, BungeeCord; Waterfall only for existing proxies, see [Networks](networks.md#proxies-and-forwarding)).
Container labels are the agent's only state, so servers keep running while an agent restarts.

## Console

Each server has a live console in the panel: its output streams in as it happens, and commands go to game servers
through their console port (RCON), and to proxies through their own console, whose answer follows in the output. The
agent keeps one console connection per game server for all its commands, so the server doesn't log a new one for each.
Commands typed in quick succession run one after the other, in their order. Proxies created by earlier versions accept
commands once they were created again, e.g. by saving their settings.

## File manager

The file manager of a server browses its data, uploads files by drag and drop (up to 16 GB each, streamed through the
master, as long as 1 GB stays free on the node, like for backups), edits configuration files in the browser and
downloads files or whole folders as ZIP archives. If something changed or deleted a file while it was open in the
editor, e.g. a plugin, a file set or another user, saving shows the difference to the file on the server and offers to
load that version or to overwrite it. Agents tell the editor the version of a file by when it was modified and its
size; with agents of older versions, saving overwrites the file as before.

Secrets such as the RCON password and the forwarding secret of a network never reach the panel. The file manager hides
files that only hold secrets (`.rcon-cli.env`, `.rcon-cli.yaml`, `forwarding.secret`, Floodgate's `key.pem`) and shows
`server.properties`, `config/paper-global.yml`, Geyser's `config.yml` and the configuration of the forwarding mods of
networks with their secrets as `<hidden>`, which saving keeps. Downloads of folders and backups leave them out the same
way, as they do with the files that [file sets](library.md#file-sets) filled secrets into. Plugins and mods run with the
server, though, and can read them.

## server.properties

`server.properties` can be edited as a form: grouped settings with switches, choices and validated numbers, a MOTD
editor with colour codes and preview, and a search. Only properties of the server's Minecraft version are shown,
comments in the file are kept, and properties the manager relies on (container port, RCON) are locked.

## Settings and images

The settings of a server can be changed after it was created: name, Minecraft version, the version of the mod loader of
Fabric, Quilt, Forge and NeoForge servers (the newest unless set), memory, port, Java version (8, 11, 17, 21, 25 or the
newest), when it starts on its own, Aikar's flags, JVM options and a CPU limit. The agent creates the container again
with the same data; the old container is only removed once the new one exists. A server keeps the image it was created
with; **Update image** in its settings pulls the newest one and, if it changed, creates the container again the same
way. The old image is removed once no server uses it. New servers get the newest image too: creating one pulls it, which
downloads its changes if it was updated since. Deleting servers keeps their images.

## Crashes

A server that crashed and starts again shows as **crashing**, with how often it crashed and its exit code. After 5
crashes in a row, each within 10 minutes of its start, the agent stops it, as Docker would start it again forever.

## Copies

A server can be duplicated on its node: the copy gets all files, worlds and settings under a new name and port, and
starts stopped. A running game server first writes its worlds to disk and pauses saving while they are copied, so
players stay connected. The copy doesn't take over the original's place in a network.

## Moving to another node

A server can move to another node with its ID, files, settings and, if chosen, its backups; otherwise the backups are
deleted with it. Port, storage location, memory and CPU limits are checked on the new node first. The server then stops,
its data is copied through the master, and it starts on the new node if it ran before. Its backup jobs, schedules, the
scopes of groups and its usage history follow it, and its network is configured again, which restarts the proxy. The
original is deleted only once the server is complete on the new node; if anything fails before, the copy goes away and
the server runs where it was. While it moves, the panel shows the progress, refuses changes to the server and continues
on the new node once it is done; scheduled tasks leave it out meanwhile. The new node needs free space for the archive
of the data besides the data itself, until it is extracted. If the master stops during a move, the server stays on its
old node, stopped.

## Lists, tags and bulk actions

The **Servers** page and the page of each node list servers as cards or as a compact table, the table from 13 servers on
until one is chosen. They are searched, filtered by state, type, node, network and tag, sorted by name, state, players,
CPU, memory or node, and grouped by network, node, type or tag, in groups that fold away. The address keeps all of it,
so that a view can be shared or bookmarked. Selected servers start, restart or stop together, run a console command such
as `save-all`, or get and lose tags; an action applies to the selected servers in a fitting state on which the user may
do it, at most 8 at a time on each node, and the panel tells which failed.

Servers have **tags** such as `lobby` or `bedwars`: up to 10, each of up to 24 letters, digits, `-` and `_`. The master
keeps them; they follow a server that moves, copies get them, and they go with a deleted server. Changing them needs the
permission to change the server's settings, though it doesn't restart the server.

## Node settings

Each node has settings for its servers: the storage location preselected for new servers, a port range (new servers get
the first free port in it) and a memory limit, so that servers together can't get more memory than the node has minus a
reserve for the system (1 GB unless changed). A server counts with the limit of its container, which gives Java about a
quarter more than the server's memory and 256 MB for what it needs besides the heap, e.g. 1.5 GB for 1 GB. Name and
agent address can be changed too.
