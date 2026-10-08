# Databases

A [network](networks.md) gets datastores: MariaDB or PostgreSQL servers that the agent of a node of its choice runs from
the official `mariadb` and `postgres` images, for plugins that keep their data in SQL and share it across the network,
e.g. LuckPerms, LiteBans, Plan or CoreProtect (which only speaks MySQL). A network has up to 10 datastores, and each up
to 50 databases, each with a user of the same name that has rights on that database only.

## Creating

The network's **Databases** tab creates a datastore with a name, an engine, a node, a storage location and its memory,
which counts against the node's memory limit like that of servers. It gets the newest major version the agent knows
(MariaDB 11.8 and 12.3, PostgreSQL 17 and 18). Its data lives in `<location>/datastores/<id>/data-<version>`, owned by
the image's user.

## Reaching a datastore

The network's servers on the datastore's node, the proxy included, join its internal network `noryx-db-<id>`,
which has no route to the internet, while they run, and keep it when their container is created again; they reach it by
the name of its container. Servers of other nodes reach it over the [private network of the
nodes](networks.md#private-network) if both nodes are part of it: the datastore's node publishes its port there, for the
nodes of the network's servers only, once the network is applied, e.g. after a node joined; a node that leaves applies
its networks again. Without the private network, servers of other nodes can't reach it, and the file sets that use it
tell why. Moving a server to a node that can't reach the datastores of its network needs a confirmation.

## Connection

**Connection** on the tab shows what to enter into the configuration of a plugin: the host and port for the servers on
the datastore's node (`noryx-db-<id>` and 3306 or 5432), those for the servers of other nodes once it is published (the
node's address in the private network and the datastore's port, which it keeps), and the database and its user, which
share their name. The master generates the password of each database: 32 characters from `a-z2-7`, which need no quoting
in YAML, TOML, HOCON or properties files. Only those who may manage datastores see it, once they ask for it. A [file
set](library.md#databases) fills in all of them for each server, e.g. `{{datastore:main.luckperms.host}}`, the password
out of sight. **New password** gives a user another one, which the plugins that use the database need then; the file
sets that use it show their servers as outdated until they are applied again.

## Browsing

**Browse** looks into a database: its tables with their estimated rows and size, and their columns and rows 50 at a
time, in the order of the primary key, with values cut to 200 characters and binary ones in hexadecimal. It only reads,
also only for those who may manage datastores.

## Log

**Show log** on the tab follows the log of the datastore's container: what MariaDB or PostgreSQL writes, e.g. why it
doesn't start or why its health check fails, which the message about a failing health check links to. It starts with the
last 300 lines and continues when the datastore starts again. The agent hides the passwords in the statements that the
engines log, e.g. when one fails: `ALTER ROLE shop PASSWORD '…'` shows as `ALTER ROLE shop PASSWORD '<hidden>'`, and
the same for `PASSWORD('…')`, `IDENTIFIED BY '…'` and `USING '…'`. Only those who may manage datastores see the log. On
the node and in the panel's terminal, `noryx-agent datastore logs <id>` follows it too, after the last 100 lines or as
many as `-n` asks for, up to 1000.

## Backups

**Back up now** and backup jobs dump the databases into a ZIP archive with one `<database>.sql` each
(`mariadb-dump --single-transaction`, `pg_dump`), kept next to the backups of servers in
`<backups of the location>/datastores/<id>`, while the datastore keeps running; **Back up now** asks for a label and the
databases, all at first. Restoring asks for the databases of the dump, all at first, creates them again and loads them
as each database's own user; one that was dropped since has to be added again first. The plugins that use them lose
their connection meanwhile, so their servers are best stopped first. Dumps can be downloaded. Locally,
`noryx-agent datastore list|backup|backups|restore` works without the master, and the panel's terminal does all but
restore.

## Upgrades and changes

The settings change the memory and CPU limits, download the newest image of the version, or move the datastore to a
newer major version: the agent dumps the databases, creates the datastore again on new data, loads the dumps and creates
the users again with the hashes of their passwords, while the plugins can't reach the databases. A failure goes back to
the old version, and its data stays until it is removed on the tab. The overview lists datastores that are unhealthy, or
stopped while servers of their network run.

## Deleting

Deleting a datastore, once its name is typed, removes its container, network, data and dumps. A network with datastores
can't be deleted.
