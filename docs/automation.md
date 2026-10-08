# Backups and schedules

The **Automation** of the panel, and how to back up the master itself.

## Backups

Backups are ZIP archives that the agent keeps on the server's node, in the `backups` folder of a storage location
(`<data-dir>/backups` by default). What a backup contains is chosen per backup or job: worlds (every folder with a
`level.dat`, also those added later), plugins or mods with their settings, configuration (the files in the server's
folder except jars and logs, and `config/`), everything, or further files and folders, chosen in a browser of the
server's folders (of one of a job's servers) or typed in. Files and folders inside them can be left out the same way,
e.g. the tiles of a map plugin such as Dynmap or BlueMap, or the logs, which can be most of a server's data; restoring
the backup leaves them as they are. A running game server writes its worlds to disk first and pauses saving while they
are archived, so players stay connected.

- **By hand.** The **Backups** tab of a server backs it up now, e.g. before an update, and lists, downloads, restores
  and deletes its backups and changes their labels.
- **Uploaded.** **Upload** on the **Backups** tab adds a ZIP archive as a backup of the server, e.g. one downloaded
  before, also after the server was deleted, or files of a server from elsewhere; it can then be restored like any
  other. It streams through the master like an upload of the file manager, up to 16 GB, as long as 1 GB stays free. The
  agent checks it like an archive of the file manager before it keeps it, and refuses archives with links, paths
  outside the server's folder, `./` or backslashes in paths, more than 100,000 entries or too much data. Uploaded
  backups are marked **From elsewhere**, and restoring one treats it as untrusted: it replaces the files and folders at
  the top of the archive, checks the archive again, leaves out the files that only hold secrets and those of the agent,
  such as the manifest of file sets, and replaces every secret in the other files with the server's own, besides
  keeping how the server takes part in its network as any restore does. Uploading needs the permissions to back up and
  to restore the server's backups, and to change its files, as a backup can bring any file, e.g. a plugin, like the
  file manager.
- **Jobs.** The **Backups** tab of the **Automation** schedules backup jobs for nodes, servers, tags and networks at
  set times, see [Targets and times](#targets-and-times). A job keeps its newest backups per server, and the newest of
  each of the last days, weeks and months that have backups, in its time zone, e.g. 7 daily and 4 weekly ones; once it
  made a new backup, it deletes the others. Days without a backup don't count, so a job that couldn't back up for a
  while doesn't delete more. Backups made by hand are never deleted that way, nor backups of a job that the **Backups**
  tab of their server marks to keep, e.g. the one before a big update, until they are no longer kept. A job backs up one
  server per node at a time, and skips servers without any of the selected data yet, e.g. new ones that never started:
  its runs list them. A job can also back up [datastores](databases.md) with all their databases, also without any
  server, one at a time per node together with the servers there; it keeps their backups the same way. The **Backups**
  tab of a server names the jobs that cover it.
- **Restoring** replaces what a backup contains with its backed up state: a backup of the worlds restores the worlds and
  leaves plugins and settings alone. All of a backup is restored, or only files and folders chosen in a browser of it,
  e.g. a single world or the folder of one plugin. The panel backs up what the restore replaces first, as a backup made
  by hand next to the restored one, unless that is turned off, so restoring the wrong backup can be undone. The archive
  is extracted next to the data first, and backed up while the server keeps running, so a running server is only
  stopped while the files are swapped, and started again afterwards. How the server takes part in a
  [network](networks.md) stays as it is: a proxy's forwarding secret and the keys and sign-ins of Geyser and Floodgate,
  the forwarding settings in its configuration, and a game server's `online-mode` and `enforce-secure-profile`. So a
  backup from before the server left its network, or joined another, brings back neither that network's secret nor the
  trust in its proxy, nor offline mode. The files of the server's console password (RCON) and files with secrets of
  [file sets](library.md#file-sets) stay as they are too. The panel then configures the server's network again, e.g.
  the servers of the proxy, and warns if that fails; after restoring with the CLI or the terminal, **Apply again** on
  the network's page does it.
- **Into another server.** A backup can also be restored into another server of the same kind, game server or proxy,
  also on another node, e.g. to look at an old world on a [copy](servers.md#copies) without touching the live server.
  It needs the permission to see the backups of the original and to restore backups of the other server. The master
  copies the backup to the other server with the secrets hidden, like a download, restores it there and deletes the
  copy. Like a duplicate, the other server keeps its own secrets and those of its network, and its forwarding settings,
  and gets no files with secrets of file sets.
- Restoring chosen files, into another server or with a backup first needs an up-to-date agent on the node, as does
  moving backups that leave something out or are kept; the panel says so otherwise. Until a node's agent is updated,
  jobs that keep backups of days, weeks or months keep all of them there, and backups there leave nothing out.
- Deleting a server deletes its backups too, but not their [copies](#copies-of-backups). Locally,
  `noryx-agent backup list|create|restore` works without the master, e.g. to restore a server while the master is
  unreachable.
- **Databases** are backed up as SQL dumps instead, see [Databases](databases.md#backups).

## Copies of backups

A node that loses its disk loses the backups on it too. So a backup job can copy each backup of a server that it makes
away from the server's node, as chosen in **Copy to**:

- **S3-compatible storage**, such as AWS S3, Backblaze B2, Cloudflare R2, Wasabi or MinIO. **Storage for copies** on
  the **Backups** tab of the **Automation** adds one: its endpoint (the host, with a port unless it is 443, e.g.
  `s3.eu-central-1.amazonaws.com`), region, bucket, an optional folder in it and an access key with its secret key,
  which needs to put, get and delete objects there. The master only connects over HTTPS, and saves a storage only once
  it could write and delete a small object (`.noryx-check`) there. Copies are kept as
  `<folder>/<server ID>/<backup ID>.zip`. **Path style** addresses the bucket in the path, for MinIO and other storage
  without a host name per bucket. **Encrypt** asks the storage to encrypt the copies with its own keys
  (`x-amz-server-side-encryption: AES256`); turn it off for storage that refuses this. The master doesn't encrypt the
  copies itself, so the storage's provider can read them, see [Security](security.md#copies-of-backups).
- **Another node**, which keeps the copies in a storage location, apart from the backups of its own servers
  (`<backups of the location>/copies/<server ID>`). The master relays them, as agents never connect to each other.

After it backed up a server, the job copies those of its backups of the server that have no copy there yet, also those
that a run before couldn't copy. Then it deletes the copies it no longer keeps: those whose backup is no longer on the
node and that its [retention](#backups) doesn't keep among the copies; copies of kept backups stay. So the job keeps as
many copies as backups, and a node that lost its backups takes none of their copies with it. The page of the job lists
the copies each run made and deleted, and a run whose copies failed fails. The master streams the copies, so it needs
no space for them. Dumps of datastores aren't copied.

A copy is a download of the backup: the files that only hold secrets of the server, such as its console password, the
forwarding secret of its network, Floodgate's key and the files with secrets of [file sets](library.md#file-sets), are
left out, and other files show `<hidden>` in place of secrets. Everything else is in it: worlds, plugins with their data
and configuration, which can hold passwords that weren't set through file sets.

- **Restoring.** The **Backups** tab of a server lists its copies after its backups, also those made while it was on
  another node. **Restore** relays a copy into the server, on its current node, and restores all of it like a backup
  of another server: the server keeps its own secrets and those of its network, and its forwarding settings. What the
  restore replaces is backed up first unless that is turned off. A copy can also be restored into another server of
  the same kind. **Copies of servers that are gone**, deleted or on a node that is offline or removed, are listed on
  the **Backups** tab of the **Automation**, from where they can be restored into another server, e.g. a new one on
  another node.
- **Deleting.** Deleting a server or a job keeps its copies, so that a lost server can still be restored; the lists of
  copies delete them. Copies to a place that the job no longer copies to stay too. Removing a node forgets the copies
  it keeps. A storage can only be deleted once no job copies to it; the master forgets its copies then, which stay in
  the bucket.
- **Permissions.** Adding, changing and deleting storages, and saving a job that copies, need the permissions to manage
  backup jobs and to see and download the backups of all servers, as copies take the backups away from their nodes. Each
  run checks that the user who saved the job last still has them, like [schedules](#schedules) that back up first.
  Seeing and restoring the copies of a server needs the permission to see its backups, those of servers that are gone
  the permission on all servers; restoring needs the permission to restore backups of the server restored into, and
  deleting a copy the one to delete backups.
- Copying to a node needs an up-to-date agent there, and restoring a copy one on the node of the server restored into.

## Backing up the master

The master keeps users, nodes, networks, templates, file sets with their secrets, backup jobs, the storages for
[copies of backups](#copies-of-backups) with their secret keys, schedules, settings and the log in its database, and the
certificate authority (CA) that its agents trust in `pki`. Losing them means enrolling every node again, and losing
track of the copies of backups. `sudo -u noryx noryx-master backup <file>` saves both in a `.tar.gz` archive, also while
the master runs; with `-` instead of a file, it writes the archive to stdout, e.g. for
`ssh master 'sudo -u noryx noryx-master backup -' > master.tar.gz` on another machine. The CA's private key lets anyone
control the agents, and the secret keys of storages let anyone read the copies there, so keep the archive as safe as the
master. To restore it, e.g. on a new machine after
`install.sh master`:

```sh
sudo systemctl stop noryx-master
sudo rm -f /var/lib/noryx-master/master.db-wal /var/lib/noryx-master/master.db-shm
sudo tar -xzf master.tar.gz -C /var/lib/noryx-master
sudo chown -R noryx:noryx /var/lib/noryx-master
sudo systemctl start noryx-master
```

The nodes keep working with the restored master. If its IP address changed, allow the new one on port 7443 of the nodes.

## Schedules

Schedules rule servers at set times, on the same [targets](#targets-and-times) as backup jobs:

- **Restart**, e.g. every night at 4:00. Players are warned in the chat beforehand (10, 5 and 1 minutes before by
  default, with an editable message) and the servers restart at the scheduled time. **Server by server in networks**
  restarts the running game servers of each network a few at a time instead (1, 2, 5 or 10), like
  [Restart server by server](networks.md#restart-server-by-server): their players move to another server of the
  network instead of being kicked, and the network's proxy restarts after them, if it is among the servers. Other
  servers restart at the scheduled time.
- **Stop** and **start**, e.g. for opening hours. Stopping warns the players like restarting.
- **Console commands**, e.g. a broadcast every evening: up to 20, which each game server runs one after the other.
- **Update image**, like **Update image** in a server's settings: each server gets the newest image of its software, and
  a running one whose image changed restarts with it.
- **Update plugins**: the plugins and mods of each server get their newest release that suits the server, never a beta
  or alpha, as in the **Plugins** tab; projects kept at their version and turned-off files are left alone. The servers
  load the new files when they restart next, e.g. with a restart schedule after it.

Restarts and stops only concern running servers, starts only stopped ones, console commands only running game servers.
The header of a server's page shows the active schedules that cover it, also through its tags and network.

**Players.** A schedule can leave servers with players alone: **Only without players** acts only on the servers without
players at the scheduled time, and **Once the players left** waits for each server's players to leave, up to a time
limit of 1 to 360 minutes, and then leaves the server alone. Players aren't warned then, as nobody plays on the servers
it acts on. The counts are those of the latest measurement of the node, which also shows on the server's page; a proxy
counts the players of its whole network, and a server whose players can't be counted, e.g. as it is just starting, is
left alone too. Waiting doesn't hold up other schedules or backup jobs, and it ends when the schedule is paused or
deleted; a run of the same schedule that is due meanwhile is skipped. With **Server by server in networks**, the game
servers of a network restart whatever their players, as these move to another server of the network first; the
network's proxy, which disconnects everyone, follows the condition with the players of the whole network.

**Back up first.** A restart, stop or update can back up each server first, as a [backup job](#backups) would: with a
selection, the files left out, a storage location and which of its backups to keep, labelled with the schedule's name.
A server that can't be backed up is left alone, and the run fails; one without any of the selected data yet, e.g. a new
one, goes on without a backup. With warnings, the servers are backed up while the players are warned, and the action
comes once all are backed up, at the scheduled time at the earliest. With a condition, each server is backed up once it
is empty, just before the action. The game servers of networks that restart server by server are all backed up before
their network's rolling restart begins.

Backing up first and updates need the permissions they need by hand, everywhere: to back up servers, to change their
settings for images, and to manage their plugins and mods. Whoever saves the schedule or runs it right away needs them,
and so does each run the user who saved it last; the schedule's page names this user. Once that user lost one of
them, or was disabled or deleted, the runs fail until someone who has them saves the schedule again.

## Targets and times

Backup jobs and schedules run on their **targets**, which can be combined:

- **Nodes**, with all their servers, also those created later, and **single servers**. Deleted servers are removed from
  the targets automatically.
- The servers with a **tag**, e.g. `lobby`: those that have it at each run, so a server tagged later is included, and
  one whose tag is removed no longer is. A run notes a tag that no server has. Whoever may change the tags of a server
  can put it under a job or schedule this way, see [Security](security.md#permissions).
- The servers of a **network**: all of them, its game servers or its proxy, as the network has them at each run.
  **Server by server in networks** works for them as for any other target. A deleted network is removed from the
  targets; a schedule left without targets says so in its runs.

They run at one or more times of day in a time zone, which a search finds by name or offset:

- on chosen **weekdays**, or every day;
- **monthly**, on chosen days of the month, e.g. the 1st and the 15th. A month without a chosen day, such as February
  without the 30th or April without the 31st, runs on its last day instead, once even if several chosen days fall on
  it;
- on **single days**, up to 100, e.g. for an event. After the last one, the job or schedule turns itself off, and it can
  only be turned on again with a day that is still to come.

Times follow the clocks of the time zone: a time that a day lacks as the clocks are put forward runs that much later
(02:30 becomes 03:30), and one that a day has twice as they are put back runs once. The master runs backup jobs and
schedules; runs it misses while it is down are skipped, and one whose last day passed meanwhile turns itself off when
the master starts. Both can be run right away, too.

The page of each job and schedule shows its latest 50 runs: when they started and how long they took, who started them
by hand, what failed and what they left out, and each of their steps on a server, e.g. a backup and the restart after
it, with what it changed, e.g. the plugins it updated. A run whose steps all left their servers out, e.g. as players
were online, counts as skipped, not failed. The card of each job and schedule lists what its last run changed. The
**Agenda** tab of the **Automation** lists what runs now and in the next 7 days, day by day in your time zone.
