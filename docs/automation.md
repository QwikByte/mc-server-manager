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
- **Jobs.** The **Backups** page schedules backup jobs for servers or whole nodes (including servers created later): on
  chosen weekdays at one or more times of day in a time zone, which a search finds by name or offset. A job keeps its
  newest backups per server, and the newest of each of the last days, weeks and months that have backups, in its time
  zone, e.g. 7 daily and 4 weekly ones; once it made a new backup, it deletes the others. Days without a backup don't
  count, so a job that couldn't back up for a while doesn't delete more. Backups made by hand are never deleted that
  way, nor backups of a job that the **Backups** tab of their server marks to keep, e.g. the one before a big update,
  until they are no longer kept. A job backs up one server per node at a time, and skips servers without any of the
  selected data yet, e.g. new ones that never started: its last run lists them. A job can also back up
  [datastores](databases.md) with all their databases, also without any server, one at a time per node together with
  the servers there; it keeps their backups the same way.
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
- Deleting a server deletes its backups too. Locally, `noryx-agent backup list|create|restore` works without the master,
  e.g. to restore a server while the master is unreachable.
- **Databases** are backed up as SQL dumps instead, see [Databases](databases.md#backups).

## Backing up the master

The master keeps users, nodes, networks, templates, file sets with their secrets, backup jobs, schedules, settings and
the log in its database, and the certificate authority (CA) that its agents trust in `pki`. Losing them means enrolling
every node again. `sudo -u noryx noryx-master backup <file>` saves both in a `.tar.gz` archive, also while the master
runs; with `-` instead of a file, it writes the archive to stdout, e.g. for
`ssh master 'sudo -u noryx noryx-master backup -' > master.tar.gz` on another machine. The CA's private key lets anyone
control the agents, so keep the archive as safe as the master. To restore it, e.g. on a new machine after
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

Schedules rule servers or whole nodes at set times, like backup jobs:

- **Restart**, e.g. every night at 4:00. Players are warned in the chat beforehand (10, 5 and 1 minutes before by
  default, with an editable message) and the servers restart at the scheduled time. **Server by server in networks**
  restarts the running game servers of each network a few at a time instead (1, 2, 5 or 10), like
  [Restart server by server](networks.md#restart-server-by-server): their players move to another server of the
  network instead of being kicked, and the network's proxy restarts after them, if it is among the servers. Other
  servers restart at the scheduled time.
- **Stop** and **start**, e.g. for opening hours. Stopping warns the players like restarting.
- **Console command**, e.g. a broadcast every evening.

Restarts and stops only concern running servers, starts only stopped ones. The master runs backup jobs and schedules;
runs it misses while it is down are skipped. The latest run and its errors are shown with each job and schedule, and
both can be run right away. Deleted servers are removed from them automatically.
