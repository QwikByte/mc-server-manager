# Servers

Everything a single server offers in the panel. New servers can also start from a [template](library.md#templates) or a
[Modrinth modpack](library.md#modpacks).

## Software

Servers run as containers based on [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server) (Vanilla,
Paper, Purpur, Folia, Leaf, Fabric, Quilt, Forge, NeoForge) and [itzg/mc-proxy](https://github.com/itzg/docker-mc-proxy)
(Velocity, BungeeCord; Waterfall only for existing proxies, see [Networks](networks.md#proxies-and-forwarding)).
Container labels are the agent's only state, so servers keep running while an agent restarts.

## Creating and deleting

**Create server** asks for the node, name, software, Minecraft version, memory, port and storage location. The field of
the Minecraft version suggests the releases that Modrinth lists, as do the settings and templates; empty means the
latest. A game server can get a seed, a game mode, a difficulty and a world type under **World**, which start as its
template has them, or as Minecraft's defaults. They are written to `server.properties` before the first start, and the
agent checks them like other properties. Deleting a server with all its worlds asks for its name first.

The page of a server shows the address players join at, with a button to copy it: the host of its node's address with
the server's port, or its proxy's for a server of a network. Without the permissions to see that node and the networks,
it shows only the port.

## Console

Each server has a live console in the panel: its output streams in as it happens, and commands go to game servers
through their console port (RCON), and to proxies through their own console, whose answer follows in the output. The
agent keeps one console connection per game server for all its commands, so the server doesn't log a new one for each.
Commands typed in quick succession run one after the other, in their order. Proxies created by earlier versions accept
commands once they were created again, e.g. by saving their settings.

The output keeps the colours of the server and its plugins, and warnings and errors stand out. It can be searched,
narrowed down to warnings and errors, cleared and downloaded as a text file of what is shown. While scrolled up, a button
counts the new lines and leads back to the end. The console opens with the last 300 lines; **Load earlier output** adds
the earlier ones of the last 1000 lines the node keeps, and older output is in the server's log files.

The prompt keeps the commands of each server while the browser tab is open, and the arrow keys walk through them. While
typing, it suggests Minecraft's commands (or the proxy's), their arguments, the names of the players online and earlier
commands; **Tab** takes a suggestion. Agents of older versions send the output without colours.

## File manager

The file manager of a server browses its data, uploads files and whole folders by drag and drop or with **Upload** (up
to 16 GB each and 10,000 files at once, streamed through the master, as long as 1 GB stays free on the node, like for
backups), creates files and folders, edits configuration files in the browser and downloads files or whole folders as
ZIP archives. A folder can be filtered by name and sorted by name, the largest or the newest first; the browser keeps
the order for every folder. **Move to…** in the menu of a file or folder moves it into another folder. Selected files
and folders are downloaded as one ZIP archive, moved or deleted together; agents of older versions can't download
several of them at once.

If something changed or deleted a file while it was open in the editor, e.g. a plugin, a file set or another user,
saving shows the difference to the file on the server and offers to load that version or to overwrite it. Agents tell
the editor the version of a file by when it was modified and its size; with agents of older versions, saving
overwrites the file as before. Saving a JSON or YAML file with a syntax error, which servers and plugins may fail to
read, asks first and names the line. The editor highlights JSON, JSON5, YAML, properties, TOML, INI-like files
(`.conf`, `.cfg`, `.ini`), shell scripts, JavaScript and XML.

Logs (`.log` files) open read only in a viewer that highlights warnings and errors. Of a large log, it shows the last
2 MB and loads earlier parts on request; with agents of older versions, large logs can only be downloaded. Archived logs
(`.log.gz`) are unpacked in the browser, up to their first 16 MB.

Secrets such as the RCON password and the forwarding secret of a network never reach the panel. The file manager hides
files that only hold secrets (`.rcon-cli.env`, `.rcon-cli.yaml`, `forwarding.secret`, Floodgate's `key.pem`) and shows
`server.properties`, `config/paper-global.yml`, Geyser's `config.yml` and the configuration of the forwarding mods of
networks with their secrets as `<hidden>`, which saving keeps. Downloads of folders and backups leave them out the same
way, as they do with the files that [file sets](library.md#file-sets) filled secrets into. Plugins and mods run with the
server, though, and can read them.

## server.properties

`server.properties` can be edited as a form: grouped settings with switches, choices and validated numbers, a MOTD
editor with colour codes and preview, and a search. Only properties of the server's Minecraft version are shown,
comments in the file are kept, and properties the manager relies on (container port, RCON) are locked. Next to the
MOTD, an image becomes the server's icon in the server list: the browser scales it to 64×64 pixels and writes it as
`server-icon.png`, which the server shows after its next start. The configuration of proxies offers the icon too. It
needs the permission to change files.

## Settings and images

The settings of a server can be changed after it was created: name, Minecraft version, the version of the mod loader of
Fabric, Quilt, Forge and NeoForge servers (the newest unless set), memory, port, Java version (8, 11, 17, 21, 25 or the
newest), when it starts on its own, Aikar's flags, JVM options, a CPU limit, the stop timeout and the time zone. The
agent creates the container again with the same data; the old container is only removed once the new one exists. A
server keeps the image it was created with; **Update image** in its settings pulls the newest one and, if it changed,
creates the container again the same way. The old image is removed once no server uses it. New servers get the newest
image too: creating one pulls it, which downloads its changes if it was updated since. Deleting servers keeps their
images.

The **stop timeout** is how long a server may take to save its worlds when it stops or restarts before it is killed:
from 30 seconds to 10 minutes, 1 minute unless changed, e.g. longer for a large modded world. The **time zone**, one of
the IANA time zones such as `Europe/Berlin` chosen from a searchable list, sets the time of the server's log and of
plugins that work with times; servers run in UTC unless one is chosen. Agents of earlier versions keep 1 minute and UTC,
which saving the settings tells.

Stopping and restarting a server run as [operations](panel.md#operations), as they may take as long as its stop
timeout: the panel follows them in a notification, and a reverse proxy in front of the master, e.g. nginx, which gives
up after 60 seconds by default, doesn't cut them off. Starting a server only takes a moment and answers right away.

When a node shuts down or reboots, systemd gives Docker 90 seconds to stop (its `DefaultTimeoutStopSec`), which cuts
longer stop timeouts short: servers that haven't saved their worlds by then are killed. To give them their full stop
timeout, raise `TimeoutStopSec` of `docker.service` beyond the longest one, e.g. to 11 minutes:

```sh
sudo mkdir -p /etc/systemd/system/docker.service.d
printf '[Service]\nTimeoutStopSec=11min\n' | sudo tee /etc/systemd/system/docker.service.d/stop-timeout.conf
sudo systemctl daemon-reload
```

## Crashes and health

A server that crashed and starts again shows as **crashing**, with how often it crashed and its exit code. After 5
crashes in a row, each within 10 minutes of its start, the agent stops it, as Docker would start it again forever. Each
crash is a warning in the log, which the bell counts, and the notice on the server's page links to its crash reports.

The images check the health of their server. A server that runs but fails its health check, e.g. as it hangs, shows as
**unhealthy** on its card and page and under what needs attention on the overview; the log tells when it becomes
unhealthy and healthy again. It still counts as running: its console, restarts, stops and schedules work as usual.

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
CPU, memory or node, either way round, and grouped by network, node, type or tag, in groups that fold away. A click on a
header of the table sorts by its column, and another click turns the order around. The address keeps all of it, so that
a view can be shared or bookmarked; where it doesn't say, the view, sort and grouping a user chose last apply, in all
lists and all their browsers. **Export CSV** downloads the servers as listed, with their node, network, type, version,
port, state, tags and what running servers use, for spreadsheets. Selected servers start, restart or stop together, run
a console command such as `save-all`, or get and lose tags; an action applies to the selected servers in a fitting state
on which the user may do it, at most 8 at a time on each node, and the panel tells which failed and offers to try those again.

Servers have **tags** such as `lobby` or `bedwars`: up to 10, each of up to 24 letters, digits, `-` and `_`. The master
keeps them; they follow a server that moves, copies get them, and they go with a deleted server. Changing them needs the
permission to change the server's settings, though it doesn't restart the server.

Servers have **notes** too, e.g. what a test server is for or whom to ask about it: up to 500 characters of plain text,
which the server's page shows and the search of the lists and **Ctrl+K** find. **Notes…** in the menu of a server
changes them. The master keeps them like tags: they follow a server that moves, copies get them, and changing them needs
the permission to change the server's settings but doesn't restart it.

## Node settings

Each node has settings for its servers: the storage location preselected for new servers, a port range (new servers get
the first free port in it) and a memory limit, so that servers together can't get more memory than the node has minus a
reserve for the system (1 GB unless changed). A server counts with the limit of its container, which gives Java about a
quarter more than the server's memory and 256 MB for what it needs besides the heap, e.g. 1.5 GB for 1 GB. Name and
agent address can be changed too.
