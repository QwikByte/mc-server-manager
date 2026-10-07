# Templates, file sets, plugins and mods

The **Library** of the panel: what new servers start with, and what many servers share.

## Templates

A template preconfigures new servers: software, Minecraft version, memory, the
[settings](servers.md#settings-and-images) of a server, `server.properties` and a list of plugins or mods from Modrinth.
When a server is created from a template, only the node, name, port and storage are chosen; `server.properties` is
written before the first start and each plugin is installed in the newest release that suits the server, so templates
don't go stale. Templates are created from scratch or from an existing server ("Save as template"), which takes its
settings, its properties (except those the manager sets) and the plugins that come from Modrinth. Worlds and plugin
configurations are not part of templates; [file sets](#file-sets) share the configurations of plugins among servers.

## File sets

A file set keeps text files that many servers share in one place, e.g. the configuration of LuckPerms, chat or
anti-cheat plugins, and puts them on the servers of tags and networks: those with a tag, or the game servers or the
proxy of a network. Paths are relative to the server's folder, e.g. `plugins/LuckPerms/config.yml`.

### Files

The page of a set is one workspace: its files as folders next to the editor, with an **Insert** menu for variables and
secrets, and beside them the set's targets, secrets and details. A bar above tells what the set needs, e.g. servers that
are outdated or secrets without a value, and offers to apply it. **Import** browses the folders of a server and takes
chosen files or whole folders at the same paths, as the file manager shows them, skipping binary files and those larger
than 1 MiB; a new file's folder can be chosen on a server too. A set keeps the newest 20 versions with who saved them;
the history shows what each version changed and loads an older one into the editor to save it as the newest. A save
based on an older version than the newest is refused, so that it can't undo what someone else saved. A set has up to 100
text files (UTF-8 without NUL bytes) of up to 1 MiB, 3 MiB in all. A file can be written only to servers that don't have
it, for files that plugins rewrite. Files that Noryx writes itself (`server.properties`, `eula.txt`, `velocity.toml`,
BungeeCord's `config.yml`, `spigot.yml`, `config/paper-global.yml`, the lists of players, `noryx-*` files), those with
secrets of the server and `.jar`, `.zip` and `.class` files can't be part of a set; plugins come from the **Plugins**
page, with their hashes checked. On a server, a path comes from one set only.

### Placeholders

The master fills in `{{server.name}}`, `{{server.id}}`, `{{server.port}}` and `{{network.server}}`, the server's name in
its network, for each server; these only hold letters, digits and a few other characters, so they can't add lines to a
file. `{{secret:<name>}}` is a secret of the set: a single line of up to 1 KiB, typed in or generated randomly. The API
only tells the names of secrets and when they changed, never their values. Other text in double braces stays as it is,
as some plugins use it themselves. The connection of a [database](databases.md) is typed in like any other setting, its
password best as a secret.

### Applying

Saving changes no server; **Save and apply** saves and goes on to apply. **Apply** shows first what changes on each
server, with the diff of each file between the server's copy and the new version, servers with the same changes
together; files with secrets show the version the server has and the new one, with placeholders instead of values, and
whether the server's copy changed. It names the servers that get the secrets for the first time. Applying writes the
files atomically as the server's user, removes those the set no longer has unless they changed on the server, at most 8
servers of a node at a time, and leaves moving servers alone. It can then restart the running servers whose files
changed, the game servers of a network a few at a time like a rolling restart, as most plugins only read their
configuration when they start.

### State

The page of a set tells for each server whether it has the newest files, an older version or other values of its
variables or secrets (outdated), files that changed on it since, none yet, whether it is no longer a target but still
has files of the set, or whether its node can't be reached. The file manager marks the files that come from a set, as
applying it again replaces changes made there.

### Leaving

A server that loses its tag or leaves its network, and the servers of a deleted set, lose the files with secrets of the
set right away, or, if their node is offline, once it is back. Applying the set again removes the other files from
servers it is no longer for, unless they changed on them; a deleted set leaves them as they are. Copies of a server lose
the files with secrets; moving a server takes everything along.

## Plugins and mods

Plugins (Paper, Purpur, Folia, Leaf, Velocity, BungeeCord, Waterfall) and mods (Fabric, Quilt, Forge, NeoForge) are
installed from [Modrinth](https://modrinth.com), either on any number of servers at once from the **Plugins** page or
from the **Plugins**/**Mods** tab of a server. The **Plugins** page switches between plugins and mods, so a project made
for both only shows the software and servers of the chosen kind. The search filters by software, Minecraft version,
categories (e.g. economy, management, optimization) and, for mods, those players don't have to install, and sorts by
relevance, downloads, followers, newest or recently updated. The master picks the newest release for each server's
software and Minecraft version, installs the projects it requires, and replaces an older version of the same project.
Where no release suits a server, it installs the newest beta or alpha and the panel warns about it. Another version that
suits the server, betas and alphas included, can be chosen instead, also to downgrade a project. Installed files are
recognised by their hash, so the tab shows their project, version (marked as beta or alpha) and available updates, also
for files uploaded by hand; it searches, filters (updates, not from Modrinth) and sorts them, and updates all at once.
Own `.jar` files can be uploaded too. Servers load changes when they restart. On the **Plugins** page, the game servers
or the proxy of a network are chosen at once, and more than 100 servers are installed on in batches of 100, one after
the other.

### Hangar

Plugins of Paper and its forks except Folia, Velocity, BungeeCord and Waterfall can come from
[Hangar](https://hangar.papermc.io), PaperMC's plugin repository, too: the search switches between Modrinth and Hangar.
They are installed, updated, chosen in another version and kept in templates like those of Modrinth, with the plugins
they require from Hangar. Installed files are recognised by their hash on both; a file that is on both counts as
Modrinth's, and installing it from Hangar replaces it rather than adding another copy. Floodgate, which lets Bedrock
players join a network, comes from GeyserMC's download server; the panel installs it on the proxy with Geyser.

### Modpacks

A server can be created from a [Modrinth modpack](https://modrinth.com/modpacks) for Fabric, Quilt, Forge or NeoForge:
**Create server** searches the modpacks and offers the versions of the chosen one, the newest release first. The pack
decides the software, the Minecraft version and the version of the mod loader. The master downloads the pack, then the
files servers need (not those for players only) and writes them into the new server, with the files the pack brings
itself (`overrides`, then `server-overrides`), except `server.properties` and `eula.txt`. Afterwards it is a server like
any other: the **Mods** tab recognises the mods and updates them. A server that didn't get all files of its pack is
deleted again. Creating servers from modpacks needs the permission to manage plugins and mods on the node.
