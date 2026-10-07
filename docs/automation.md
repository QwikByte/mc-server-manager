# Backups and schedules

The **Automation** of the panel, and how to back up the master itself.

## Backups

Backups are ZIP archives that the agent keeps on the server's node, in the `backups` folder of a storage location
(`<data-dir>/backups` by default). What a backup contains is chosen per backup or job: worlds (every folder with a
`level.dat`, also those added later), plugins or mods with their settings, configuration (the files in the server's
folder except jars and logs, and `config/`), everything, or further files and folders, chosen in a browser of the
server's folders (of one of a job's servers) or typed in. A running game server writes its worlds to disk first and
pauses saving while they are archived, so players stay connected.

- **By hand.** The **Backups** tab of a server backs it up now, e.g. before an update, and lists, downloads, restores
  and deletes its backups.
- **Jobs.** The **Backups** page schedules backup jobs for servers or whole nodes (including servers created later): on
  chosen weekdays at one or more times of day in a time zone, which a search finds by name or offset. A job keeps the
  newest backups per server and deletes older ones; backups made by hand are never deleted that way. A job backs up one
  server per node at a time, and skips servers without any of the selected data yet, e.g. new ones that never started:
  its last run lists them. A job can also back up [datastores](databases.md) with all their databases, also without any
  server, one at a time per node together with the servers there.
- **Restoring** replaces what a backup contains with its backed up state: a backup of the worlds restores the worlds and
  leaves plugins and settings alone. The archive is extracted next to the data first, so a running server is only
  stopped while the files are swapped, and started again afterwards.
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
  default, with an editable message) and the servers restart at the scheduled time.
- **Stop** and **start**, e.g. for opening hours. Stopping warns the players like restarting.
- **Console command**, e.g. a broadcast every evening.

Restarts and stops only concern running servers, starts only stopped ones. The master runs backup jobs and schedules;
runs it misses while it is down are skipped. The latest run and its errors are shown with each job and schedule, and
both can be run right away. Deleted servers are removed from them automatically.
