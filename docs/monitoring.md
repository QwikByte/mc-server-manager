# Usage and logs

What nodes and servers use, and who did what.

## Usage

The panel shows what nodes and servers use, now and during the last week.

### Current values

Every agent measures every 5 seconds: CPU and memory of the node and of each server (without the page cache, like
`docker stats`), the network traffic of each server, the size of its data (every 5 minutes), the players online and the
ticks per second of Paper, Purpur and Leaf servers. The number of players comes from the status request that the server
list in the game sends too, which game servers and Velocity answer; BungeeCord and Waterfall are left out, as they log
every such request. The names of all players and the ticks per second come through the server's console port (`list`,
`tps`) over a connection that stays open, because servers log every new one. Game servers behind a proxy only listen
within the network of their proxy, where the status request can't reach them, so their console tells the number of
players too. A network counts the players of its game servers, like the **Players** page, or those its proxy counts
while the agents can't tell them, e.g. agents of older versions. Server cards show CPU, memory and players; the
**Usage** tab of a server and the node page show the rest.

### History

The master records the latest measurement of every agent each minute and keeps it for a week. Charts show the last 24
hours (averages of 5 minutes) or 7 days (averages of 30 minutes), with the most players and the largest size of the data
of each step, and a table shows the same values. Gaps are times in which a server didn't run or its node couldn't be
reached. The history of a server moves and goes away with it.

## Logs

The master keeps a log of what happens on it and on its agents, so that it's clear who did what and what went wrong.
Entries are kept for 30 days unless the settings say otherwise, and at most the newest million.

### Actions

Every request of the panel that changes something, and every download of a file, folder, backup or export, is logged
with the user, the IP address, the node and server it concerned (with their names at that time), the outcome and how
long it took. Denied requests are logged as warnings, and failed ones as errors if the master or an agent failed.
Others, e.g. with an invalid input, are only information: the panel tells the user why. Sign-ins, failed sign-ins,
password changes, changes of two-factor authentication, sign-outs, enrollments, certificate renewals and what backup
jobs and schedules did on each server are logged too.

### Agents

An agent logs every call it receives with its origin (the master or its local CLI) and keeps its latest entries in
memory. The master collects them over the mutually authenticated connection and continues where it left off, also after
a restart of either. Calls that only read are logged at the debug level, downloads at the info level.

### Logs page

The **Logs** page lists the entries newest first, with new ones streaming in. Filters for the time, level, category,
source, node and a text search are part of the address, so a view can be shared. An entry opens to show all its details
and narrows the list to its user, server or category. Key figures and a chart show the warnings and errors of the last
24 hours; selecting an hour shows its entries. The entries are exported as CSV or JSON lines.

### Everywhere else

A bell in the sidebar counts the new warnings and errors, and new ones show up as notifications, except those of the
user's own actions. Among them are every crash of a server, servers that become unhealthy, and nodes that go offline:
the master checks the connections to the agents every 30 seconds and logs a node that fails two checks in a row once,
and once more when it is back. Servers have an **Activity** tab and nodes an **Activity** section.

### Command line

`noryx-master logs` and the terminal's `logs` command show the log with the same filters, also as JSON lines and
following new entries with `-f`. `noryx-agent logs [-f]` shows an agent's own log, also while the master is unreachable.

### Console and files

Master and agent log to stderr as text or, with `--log-format json`, as JSON; `--log-level` chooses the least important
level (`info` by default) and `--log-file` also writes JSON lines to a file, which is rotated at 10 MB with 5 older
files kept.
