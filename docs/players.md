# Players

The **Players** page lists the players online on all game servers, with their server and network, to search and act on;
[`Ctrl+K`](panel.md#search) finds them too. Its other tabs join the ban list, whitelist and operators of all servers or
of a network, with how many servers have each player, and the changes that wait for stopped servers.

## Actions

Kick, ban (with a reason), pardon, add to and remove from the whitelist, make operator and take it away, and turn the
whitelist on or off. A change goes to the player's server, the network or all servers, as chosen; bans go to the network
first. Kicks leave the server: Velocity sends kicked players to another server of the network, BungeeCord disconnects
them. **Send to another server** moves a player within the network through the proxy (`send`). The agent reads the
proxy's answer, so the panel tells if the player isn't online or the proxy has no `send`. Velocity only answers if it
can't send, so the agent waits a second for that answer; rolling restarts don't wait. Players named `all` or `current`,
and on BungeeCord like a server of the network, can't be sent, as `send` would read their name as other players too.

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
