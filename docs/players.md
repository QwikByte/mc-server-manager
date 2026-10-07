# Players

The **Players** page lists the players online on all game servers, with their server and network, to search and act on;
[`Ctrl+K`](panel.md#search) finds them too. Its other tabs join the ban list, whitelist and operators of all servers or
of a network, with how many servers have each player, and the changes that wait for stopped servers. Temporary bans,
which Paper's ban list and plugins such as EssentialsX write, show when they end, and those that ended are marked. The
**Players** tab of a game server shows the same for that server alone, with the same actions.
**Players online** on a network's page opens the page for that network.
The headers of the tables sort them, by player, server and network or by player and number of servers; the address keeps
the order. **Export CSV** downloads a tab as shown, searched and sorted, for spreadsheets: the players online with their
server, node and network, or the players of a list with their servers, and for bans the reason, when it started and ends
and who banned them.

## Actions

Kick, ban (with a reason), pardon, add to and remove from the whitelist, make operator and take it away, and turn the
whitelist on or off. A change goes to the player's server, the network or all servers, as chosen; bans go to the network
first. Kicks leave the server: Velocity sends kicked players to another server of the network, BungeeCord disconnects
them. **Send to another server** moves a player within the network through the proxy (`send`). The agent reads the
proxy's answer, so the panel tells if the player isn't online or the proxy has no `send`. Velocity only answers if it
can't send, so the agent waits a second for that answer; rolling restarts don't wait. Players named `all` or `current`,
and on BungeeCord like a server of the network, can't be sent, as `send` would read their name as other players too.
A ban suggests the reasons already in the ban list.

## Stopped servers

Stopped servers get a change once they run again, so that a ban also reaches the servers of a network that are stopped.
The agent keeps the waiting changes in `noryx-pending-players.json` in the server's data.

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
send console commands, as operators may run any command in the game.
