# Security model

How Noryx protects the master, the agents and the servers they run, and what a compromised part can and can't do.

## Own certificate authority

The master creates an Ed25519 certificate authority on first start. All master ↔ agent traffic is TLS 1.3 with mutual
authentication. Identities are names, not IPs (`master.noryx.internal`, `<node-id>.node.noryx.internal`), so nodes can
change their address without re-enrolling.

## Short-lived certificates

Master and node certificates are valid for 90 days and renewed automatically once a third of their lifetime is left. For
a node, the agent creates the new key and only sends a signing request; it installs the signed certificate after
checking it, without a restart. The panel can renew a node on demand. A node that stays offline until its certificate
expires has to be enrolled again with a new join token. So that the panel warns about such a node in time, also after
the master restarted, the master stores when each node's certificate expires: from the certificates it issues and those
a node presents when it connects, which the CA signed for that node, and only when it is later than the stored expiry.
A compromised node can't make its certificate seem to expire sooner or later than the latest one issued.

## Enrollment

Adding a node creates a single-use join token (valid for one hour unless the settings say otherwise, stored only as a
hash). It contains the master address, the node ID, the secret and the CA fingerprint. The agent creates its key pair
locally, sends a CSR and pins the CA fingerprint, so the exchange can't be intercepted. Private keys never leave the
node. The master checks the token before it signs the CSR, and a client gets 5 attempts, then one every 12 seconds.

## Agents only obey the master

The agent requires a client certificate with the master identity. Node certificates are server-only, so a compromised
node can't command other nodes. Locally, the agent is controlled through a Unix socket (mode `0600` inside a `0700` data
directory).

## Panel

Argon2id password hashes, session tokens stored as SHA-256 hashes, `__Host-` cookies (`HttpOnly`, `Secure`,
`SameSite=Strict`), cross-origin request protection, a strict Content Security Policy and self-hosted fonts. Sign-in
attempts are rate limited per client address (IPv6 per /64 network) and per username, changes that need the password per
user. A username has a larger budget than a client, so that a single client can't keep a user out. Client addresses come
from the `X-Forwarded-For` or `X-Real-IP` header only for the reverse proxies named with `--trusted-proxy`, also those
that sessions show. The panel names a session by a random ID of its own, never by its token or the token's hash, and
users only see and end their own sessions, which needs no password, as it only takes rights away. A session notes the
address, browser and time of its last use at most once a minute; the browser and operating system are only names from
fixed lists that the master recognises in the User-Agent, which are shown but never trusted. The CSV
files of servers and players, which the browser writes, protect spreadsheets from formulas in names, tags and ban
reasons as the export of the log does. Notes of servers are plain text, which the panel shows as text only.

## Two-factor authentication

Codes of the app (RFC 6238) work only once, and from the fifth wrong code in a row on, codes aren't checked for a minute
that doubles with every further wrong one, up to a day; parallel guesses count too. Recovery codes have 50 random bits
and are stored as SHA-256 hashes. Those who may change the master's settings can require two-factor authentication for
all users or for groups; the change is logged. Until a user it applies to has set it up, the master refuses every route
but those of the user's own account (setting it up, signing out, the password, sessions, language and preferences) with
403, the terminal, streams and the permissions the panel loads included. Setting it up is never refused, so the
requirement locks nobody out, and `noryx-master user add` stays the way back for administrators who lost their app and
recovery codes. The secret of the app is stored in the master's database, which needs the same
protection as the CA key next to it. The panel must be served over HTTPS (its settings, a reverse proxy or
`--tls-cert`/`--tls-key`), otherwise browsers drop the secure session cookie (`localhost` is exempt). With a certificate
of Let's Encrypt or `--tls-cert`, the master tells browsers to use HTTPS only (HSTS, one year), but not with a
self-signed one, which would lock browsers out once it changes; behind a reverse proxy, set it there. The master may
listen at ports below 1024 (`CAP_NET_BIND_SERVICE`), e.g. 443 and 80, and has no other privileges.

## Networks

Velocity's modern forwarding signs the forwarded player data with a random secret per network, which is stored in the
master's database and on the network's servers; the API never returns it, and the file manager hides it in every file
that holds it. Legacy forwarding (BungeeCord's) can be spoofed by anyone who reaches a server, so the servers of such a
network are only reachable by the proxy on its node, and those on other nodes need the operator's confirmation that a
firewall protects them. Servers that leave a network, also as it is deleted, run in online mode again without the
secret. Restoring a backup keeps what decides how a server takes part in a network as it is: a proxy's forwarding secret
and the files with secrets of Geyser and Floodgate, the forwarding settings, including the secret Velocity 1 kept in
its configuration, and a game server's online mode. The agent puts them into the extracted backup before it replaces
anything, so an old backup brings back neither the secret of a network the server left nor the trust in its proxy, nor
a former secret of its network; the master then configures the server's network again. A removed node may be
compromised, and its proxy's data holds the secret, so a node with servers of networks is
only removed once they left their networks: game servers of other nodes no longer trust its proxy, and proxies no
longer send players to its servers. The node itself isn't contacted, and if a server of another node can't be
configured, the node stays. Only deleting a network whose proxy's node doesn't answer skips the proxy and the servers
on that node, which then only trust each other, and says so. The forwarding mods come from Modrinth like other mods,
checked against their SHA-512 hashes. Proxies read console commands from their standard input, which only the agent
writes to through Docker; no RCON plugin is added.

## Containers

Containers run with `no-new-privileges`, memory and PID limits, and only the capabilities the images need to hand the
data to the server's user: `CHOWN`, `SETUID` and `SETGID`. Proxies run as the user of their image (uid 1000) from the
start, which owns their data, without any capabilities. Servers of a node can't reach each other: they share a Docker
network without communication between containers (`noryx-servers`), and a Velocity proxy shares another one only with
its backends on the node.

## Agent input

Every request is validated by the agent. Server files are confined to the data directory (`os.Root`), and game servers
are only created after the operator accepts the Minecraft EULA. JVM options may only contain characters that the image's
start script can't interpret as shell syntax, can't override the memory limit and can't run code: Java agents, class and
module paths, commands on errors, options read from files, class data archives, JVMCI compilers, debugging or JMX ports,
and the system properties of Java, JNDI, logging libraries and JNA, which name classes, libraries or configurations to
load (also from URLs), are refused. A server that got such an option before an update refused it keeps it until it is
removed: the agent logs a warning when it starts, and the panel shows it on the server's page and the overview.
Settings can't set other variables of the images, such as `CUSTOM_SERVER`, `PLUGINS` or `JVM_XX_OPTS`, which download or
run code and would get around these checks: besides the variables of checked settings, a container only gets its time
zone as `TZ`, once the agent found it among the IANA time zones it knows.

## File manager

The agent confines every path to the server's data directory, including through symbolic links, and new files belong to
the server's user. Downloads are sent as attachments with a sandboxing CSP, so an uploaded HTML file can't run scripts
in the panel. Secrets of the server stay on the node: files that only hold them can't be listed, read, written or moved,
others show them as `<hidden>`, no file or folder with secrets can be moved where they would show, and archives leave
them out. Only moving a server to another node copies them. Moving or deleting several files and folders at once
checks each of them like a single one. The viewer of logs shows them as text and unpacks archived logs in the browser
only up to 16 MB, so that a small archive can't exhaust the browser's memory.

## Plugins and downloads

The master downloads only from Modrinth's CDN, up to 256 MB, and only uses a file whose SHA-512 hash matches the one
Modrinth's API lists; from Hangar, only from its CDN and with the SHA-256 hash its API lists, and versions that only
link elsewhere can't be installed. Floodgate only comes from GeyserMC's download server, with the SHA-256 hash its API
lists. A modpack is checked the same way, and each of its files against the SHA-512 hash in the pack; packs with files
elsewhere than on Modrinth's CDN or with paths that leave the server's folder are refused before a server is created,
and the agent confines the files like those of the file manager. The version of a mod loader ends up in a variable of
the server image, so the agent only accepts letters, digits, `.`, `_`, `+` and `-`. The agent decides the folder from
the server type and only accepts plain `.jar` file names in it. Project icons are fetched by the master, so the browser
never contacts Modrinth or Hangar and the Content Security Policy stays unchanged. To whitelist a Bedrock player, the
master sends their gamertag to GeyserMC's global API, and the agent only accepts the IDs Floodgate gives Bedrock
players.

## File sets

Applying a set needs the permission to change the files of every server it touches, besides the one to manage file sets,
as a set can configure plugins that run code, e.g. scripts; whoever may only change the tags of a server can make it a
target, but never applies anything, and the preview names the servers that would get secrets for the first time. Secrets
are stored in the master's database like the forwarding secret, as a key next to them would be in the same backups as
the CA key, which already controls all agents. The master sends their values only to the agents, in a field of their own
that, like the files, is never logged; the agent fills them in and hides the resulting files like the RCON password: the
file manager can't list, read, write or move them, and downloads of folders and backups leave them out. Each server
records in `noryx-filesets.json` which set wrote which file, with its SHA-256 hash, and which files held secrets; these
stay hidden for the life of the server. The manifest is hidden too, can't be deleted or replaced from the file manager,
and is left out of backups and restores, so that restoring an older backup can't bring back a file with secrets without
hiding it; restoring keeps these files as they are, so that it brings back no secret of a set the server left. As a set
marks a file before it writes it, the agent checks the marks once a file is open, also while it archives a folder, and
copies of a server lose every file that the original marked. Master and agent both refuse the files that Noryx writes
itself or that hold secrets of the server, so a set can't change forwarding or RCON settings, and the agent writes no
file through a link or into a file where a folder should be. A compromised server can change its own manifest, which
only changes its own state or the hiding of secrets it can read anyway.

## Databases

A datastore runs as the image's user without capabilities, with `no-new-privileges`, memory, CPU and PID limits and a
health check. The agent generates the superuser's password, which the image reads from a file that only it may read,
never from the environment or labels, and which never leaves the node; MariaDB's root may only sign in on the container
itself. Every database has its own user with rights on it alone, and on PostgreSQL nobody else may connect to it or to
the superuser's database. Only the network's servers reach a datastore: on its node over an internal Docker network
without internet, which they share with each other as backends in a proxy's network already do; from other nodes only
over the private network of the nodes, for the nodes of its servers, never at a public address. The agent checks every
name against `^[a-z][a-z0-9_]{0,31}$`, refuses those of the engines themselves, and only takes passwords from `a-z2-7`,
so no statement or configuration file needs escaping; statements go to the clients in the container on their standard
input. Dumps are loaded as their database's own user, never as the superuser, with MariaDB's sandbox mode, so a dump
can't gain more rights; as MariaDB has no way to sign in as a user without its password, the user gets a random one for
the time of the load. The agent never stores the passwords of users: upgrades keep the hashes, which it checks before it
uses them in a statement. Dumps are kept like backups and checked before a restore drops anything, so a damaged one
changes nothing. Browsing reads as the superuser in a session that only reads and stops each statement after 10 seconds,
with statements the agent builds itself from names that need no escaping: tables, schemas and columns whose names don't
match `^[A-Za-z0-9_$-]{1,64}$` aren't shown, MariaDB's client runs in its sandbox, and a page has at most 3 MiB. The log
of a datastore's container, which only those who may manage datastores see, shows the statements that the engines log,
e.g. when one fails, so the agent hides every quoted value after `PASSWORD`, `PASSWORD(`, `IDENTIFIED BY` and `USING`,
the hashes of passwords too, before it sends a line: also with escaped quotes, and to the end of the line where it can't
tell where a value ends. It drops control characters, so that a line can't control the terminal of the local CLI, and
the master relays the log without logging it. The passwords are stored in the master's database like the forwarding
secret, and are never logged. The API returns them
only to those who may manage datastores, one at a time on request, without caching, and logs who asked; they can
download all data in dumps anyway. A compromised master knows them, as it knows the forwarding secret, and could restore
or delete data, but can't learn the superuser's password or place data outside the allowed storage locations.

## Duplicates

Copying never follows symbolic links, so a copy can't pull in files from outside the server's directory. A copied
Velocity proxy loses its forwarding secret, a copied BungeeCord proxy stops forwarding and a copied game server stops
trusting the proxy, so a copy can't impersonate a server of a network. Copied proxies also lose Floodgate's key, with
which Geyser vouches for Bedrock players, and copied game servers demand signed chat again. A copied Fabric or Quilt
server keeps FabricProxy-Lite, which turns players away until it is removed in the **Mods** tab.

## Backups

The agent keeps backups outside of the servers' folders, accessible to itself only (mode `0700`), so a compromised
server can't read or tamper with them. The master can only choose among the storage locations the node's administrator
allowed, and backup IDs and paths are validated by the agent. Restoring confines every entry to the server's folder, and
backups never contain symbolic links. Downloads are attachments like those of the file manager and hide the secrets the
same way; the backups on the node keep them, so restoring works. Restoring keeps the secrets of file sets and networks
as they are, see [File sets](#file-sets) and [Networks](#networks).

## Permissions

Every API route states the permission it needs when it is registered, so none can be added without; the terminal checks
each command the same way and refuses commands without a check. Permissions are loaded for every request, so changes,
disabling and deleting apply right away; disabled users are signed out. Streams that follow output, the console, the
log of a datastore and terminal commands such as `server logs`, end every 5 minutes, so the panel checks the session and
the permissions again; the console and the log connect again on their own and continue. Users can only grant
permissions they have themselves, within their own scope, and only manage users who have no more permissions than they
do, so no one can raise their own permissions.
The last enabled administrator can't be disabled, deleted or removed from the Administrators. The master logs every
change with the user who made it, also denied attempts. An operation in progress can only be cancelled by the user who
started it, or by one who has the permissions it needed on what it is about, and only while it is at steps that stop
safely: restoring a backup, moving a server, stopping or restarting one and restarting servers one after the other
always finish.

## Terminal

The panel's terminal is not a shell. The agent's commands are the same code as its local CLI, but run in the master and
reach the agent through the existing mutually authenticated connection, so the agent offers nothing new to the master.
Commands that only the node's administrator may run (`storage`, `enroll`, `overlay allow`, `deny` and `up`) don't exist
there. Command lines are split like a shell would, but nothing is expanded or executed by one, and the master logs every
command with the user who ran it. Completion offers only the commands that the user's permissions allow on some server
or node of the target, and only the servers, nodes, datastores and backups the panel shows the user anyway; each command
still checks its permission with its arguments when it runs.

## Log

An agent can only add entries about its own node and its servers, at a limited rate of entries and of bytes, and entries
are cut to a maximum size. Besides its retention, the log is kept under a size limit (2 GiB by default) that the master
checks every minute, so that agents can only exceed it by what they add in a minute, a few MiB each. SQLite reuses the
space of deleted entries, so the log takes at most about twice its limit on disk, however long the retention. So a
compromised agent can't fill the database, also over weeks, or write entries about other nodes. Request fields that may hold
secrets, such as the forwarding secret of a network, are never logged. Exports protect spreadsheets from formulas in
entries, and log files are only readable by their owner. A live stream ends every 5 minutes and the browser connects
again, which checks the session and the permissions again. Behind a reverse proxy, the logged IP address is that of the
proxy, unless `--trusted-proxy` names it.

## Moving servers

Agents never connect to each other: the master relays the server's archive and backups between them over its mutually
authenticated connections. The new node checks the settings like those of a new server and extracts the archive confined
to the server's data directory, without symbolic links. Moving needs the permissions to delete the server and read its
files, and to create servers on the new node.

## Console

To run commands on a game server, e.g. to ask it for its ticks per second, the agent reads the console password from the
server's `server.properties` and connects to the server's console port inside Docker's network; the password never
leaves the node. A server that doesn't answer its console, e.g. a compromised one, holds up only its own commands: the
agent applies the waiting changes of players to each server on its own, and gives each change 15 seconds before it waits
for the next try.

Servers and their plugins write the output, so the panel shows it only as text: the agent turns its colours into
Minecraft's colour codes, which the panel only maps to class names of a fixed set, never to markup. The plain output,
which the CLI and the terminal show, has no control characters but line breaks and tabs, so that a server can't control
the terminal it is shown in.

## Private network

Only nodes whose administrator ran `noryx-agent overlay allow` join, so the master can't open a UDP port and an
interface on a host that didn't agree. Each node creates its WireGuard key itself
(`/var/lib/noryx-agent/overlay/private.key`, mode `0600`); only the public key reaches the master, over the node's
mutually authenticated connection, and the master's backup holds public keys only. The agent checks what the master
configures: the range must be a private IPv4 range (RFC 1918) that overlaps none of the node's addresses and routes,
e.g. a provider's private network or a Docker network, and each peer gets exactly one address in it, so a master can't
pull other traffic of the node into the tunnel. An nftables table of its own (`inet noryx`), which works next to
Docker's iptables and nftables rules, drops packets for the node's address that don't arrive through the tunnel, new
connections from the tunnel to the node itself, e.g. to SSH or the agent, and forwarded connections from the tunnel
except those of a proxy's node to the ports of its servers. WireGuard accepts from a peer only its own address and
doesn't answer unauthenticated packets. The agent binds each address it lets reach a port to the public key the peer at
that address had then, which the master sends along, and drops the address once a peer has it with another key: a node
that gets the address of a removed one, even while a member was offline and never saw the removal, reaches none of the
ports published for the removed one. A compromised node reaches only the ports of its own servers on other nodes;
removing it removes it everywhere. A rotated key is let in again as the master applies the node's networks again; a key
that changes otherwise, e.g. as a node lost its data, needs its networks applied again in the panel. A compromised
master could add a peer of its own, but it can already reconfigure every server. If the interface is missing, e.g.
after a failed boot, the ports of these servers are reachable nowhere.

## Storage locations

Only the node's administrator decides where server data may be stored (`noryx-agent storage add`). The panel can only
choose among these locations, so a compromised master can't mount other host directories into containers.

## Installation

The installer only runs once it is downloaded completely, downloads only over HTTPS from the releases and installs a
package only if its SHA-256 checksum matches the release's `checksums.txt`, and only if the release key that is part of
the installer verifies its signature. The private key is a secret of the repository that only the job signing the
release gets, so someone who can merely change releases, e.g. with a stolen token, can't make installations accept other
files. Release files also carry build provenance attestations, signed through GitHub, that tie them to the release
workflow and the tagged commit. The master runs as its own system user with a hardened systemd unit, and refuses a data
directory of another user, so a command run as root can't leave files there that lock the master out.

## Updates

The master can't install anything itself. To update, it only creates a file that makes systemd start
`noryx-master-update`, which runs as root but takes no input from the master: it installs the latest release with the
installer of the installed package, checking the signature and the checksums. Agents only install releases newer than
themselves, so even a compromised master can't downgrade a node to a vulnerable version, and the version must have the
form of a release. `systemctl mask noryx-master-update.path` forbids updates from the panel.
