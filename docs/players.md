# Players

The **Players** page lists the players online on all game servers, with their server and network, to search and act on;
[`Ctrl+K`](panel.md#search) finds them too. **Seen** lists the players who were online, those seen last first, with
when, how long and where they played. Its other tabs join the ban list, whitelist and operators of all servers or
of a network, with how many servers have each player, and the changes that wait for stopped servers. Temporary bans,
which Paper's ban list and plugins such as EssentialsX write, show when they end, and those that ended are marked. The
**Players** tab of a game server shows the same for that server alone, with the same actions.
**Players online** on a network's page opens the page for that network.
The headers of the tables sort them, by player, server and network, by when they were seen and how long they played, or
by player and number of servers; the address keeps the order. **Export CSV** downloads a tab as shown, searched and
sorted, for spreadsheets: the players online with their server, node and network, the players seen with when and how
long they played and their servers, or the players of a list with their servers, and for bans the reason, when it
started and ends and who banned them.

## A player's page

Each name opens the player's page at `/players/<name>`, which `Ctrl+K` finds too, also of players who are offline: when
the player was first and last seen, how many minutes they played on each day and on each server, their entries in the
ban list, whitelist and operators of the servers, and the actions below.

The master notes every minute which players are online on each game server, as its agent measures them anyway, and
keeps where they played for as long as the [log](monitoring.md#logs) (30 days unless the settings say otherwise). It
counts a minute for each measurement that names the player, so the time of a server whose console doesn't answer, which
then only names some players, is missing. Proxies aren't counted, as their players are those of the game servers.
Sightings move with a server to another node and go with a deleted server. Users only see the sightings on the
servers they may see.

## Actions

Kick, ban (with a reason), pardon, add to and remove from the whitelist, make operator and take it away, and turn the
whitelist on or off. A change goes to the player's server, the network or all servers, as chosen; bans go to the network
first. The notification tells on which servers a change failed and offers to try it again there. Kicks leave the
server: Velocity sends kicked players to another server of the network, BungeeCord disconnects
them. **Send to another server** moves a player within the network through the proxy (`send`). The agent reads the
proxy's answer, so the panel tells if the player isn't online or the proxy has no `send`. Velocity only answers if it
can't send, so the agent waits a second for that answer; rolling restarts don't wait. Players named `all` or `current`,
and on BungeeCord like a server of the network, can't be sent, as `send` would read their name as other players too.
**Send players elsewhere** in the [network's list of servers](networks.md#restart-server-by-server) moves all players of
a server at once, like a rolling restart does before a server restarts.
A ban suggests the reasons already in the ban list.

## Stopped servers

Stopped servers get a change once they run again, so that a ban also reaches the servers of a network that are stopped.
The agent keeps the waiting changes in `noryx-pending-players.json` in the server's data. It tries them every 5 seconds;
a server that doesn't answer a change within 15 seconds keeps it for the next try, and holds up neither the other servers
of its node nor their lists.

## How it works

The agents run Minecraft's own commands (`minecraft:ban` and so on) through the server's console port, so the lists stay
in the server's files, also where plugins replace the commands, and read the lists from those files. Names are checked
before they become part of a command: 16 letters, digits and underscores, or Floodgate's dot before.

## Bedrock players

Bedrock players have Floodgate's dot before their name. The servers behind the proxy can't look them up, so the
whitelist gets them with the ID Floodgate gives them, which the master asks GeyserMC's global API for: GeyserMC knows
the players who joined a server with Geyser before. The agent writes them into `whitelist.json` and makes the server
reload it (`whitelist reload`); stopped servers get the change once they run. Other actions work by name once a player
joined the server.

## Permissions

Acting on players needs the permission to manage players on each server; making operators also needs the permission to
send console commands, as operators may run any command in the game. Sending players to another server of a network
needs the permission to manage the players of its proxy.
