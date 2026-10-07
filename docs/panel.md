# The panel

How the admin panel is organised, and what works the same on all of its pages.

## Overview

The **Overview** is the panel's start page: the players online, the servers by state, the nodes with what they use, the
networks, the servers with the most players, and what needs attention: crashing and unhealthy servers, offline nodes,
nodes with more memory assigned than they can give or almost full storage, nodes whose certificate expires within two
weeks, proxies that are stopped while their servers run, and backup jobs and schedules whose last run failed. It counts
like the **Nodes** page: servers that run, not those that start or crash, and the memory assigned against what the
online nodes can give their servers, after the reserve. Sizes are in binary units (MiB, GiB).

A node's certificate is renewed a month before it expires while the node is online; one that stays offline until then
has to be connected again with a new join token. The master remembers the expiry of an offline node's certificate as
long as it runs; after a restart it learns it again once the node is online.

The Overview is made of widgets: key figures, what needs attention, the nodes with their CPU of the last 24 hours, the
load of all nodes over the last 24 hours, pinned servers, networks, the servers with the most players, the latest
entries of the log, the next runs of backup jobs and schedules with those whose last run failed first, and quick
actions to create a server or a network or add a node. Each user only gets the widgets
their permissions allow. **Customize** arranges them: widgets are dragged by their handle to the place of another, or
moved a place with the arrow keys on it, span one, two or all three columns, and are hidden and added again; **Reset**
brings back the default layout. The master keeps the layout for each user, like the language, so it applies in all their
browsers. The players online and the CPU load of the key figures show how they went while the panel is open.

## Navigation and pinned servers

Servers are pinned with the pin on their card, in the table or on their page. Pinned servers are listed with their state
in the sidebar and in their widget, which also starts and stops them. The master keeps them for each user too, up to 20;
they follow a server that moves and go with a deleted server or a removed node. The sidebar lists the overview, servers,
networks, players and nodes, the **Library** (templates, file sets, plugins and mods) and the **Automation** (backup
jobs and schedules), each with its parts as tabs, and the log and settings at its foot. The menu of the user's name
holds their account, the colour theme, the language and signing out. The sidebar folds to its icons, which each browser
remembers. Lists, figures, charts, tabs and pages are animated, unless the operating system asks for less motion.

## Settings of each user

Each user chooses on their account page, in the menu of their name, the colour theme, the accent colour, the density,
whether times have 24 or 12 hours and the language of the panel; the menu itself offers the colour theme and the
language too. The accent gives buttons, links and highlights their colour: emerald, blue, violet or graphite, each
readable in both themes (WCAG AA), while the colours of states, such as running or failed, stay. The compact density
tightens all spacing, e.g. for long lists of servers, on screens used with a mouse or touchpad; touch screens keep the
comfortable one.

The master keeps these choices for each user, as well as the view, sort and grouping of server lists chosen last, so
that they apply in all their browsers. Each browser remembers the look and the clock it showed last and uses them until
someone signs in; what a user never chose follows the browser. `GET /api/preferences` returns a user's settings with the
layout of their overview and their pinned servers, and `PATCH /api/preferences/settings` changes some of them, e.g.
`{"theme": "dark"}`, or takes one back to the browser's with `null`. The master refuses settings and values it doesn't
know.

## Search

**Ctrl+K** (⌘K) or `/` searches servers, also by tag and notes, players online, networks, nodes and pages from anywhere in the
panel, and, once something is typed, templates, file sets, backup jobs, schedules and users; each only for those who may
see them. Before anything is typed, it offers what was opened last: servers, networks, nodes, templates, file sets,
backup jobs and schedules, wherever they were opened. Each browser remembers them for each user, and only those the user
may still see are offered. A search that starts with an action, e.g. `restart lobby`, starts, restarts or stops a server
or opens its console. **Create server**, **Create network** and **Add node** open their dialogs from the search too.

## Keyboard shortcuts

Shortcuts are keys typed one after the other, e.g. `g` and then `s` for the servers. `?` lists those the user may use.
They do nothing while the focus is in a field or the editor, or while a dialog is open; only **Ctrl+K** works there too.

| Keys                              | Opens                                                  |
| --------------------------------- | ------------------------------------------------------ |
| **Ctrl+K** (⌘K), `/`              | the search                                             |
| `?`                               | the list of shortcuts                                  |
| `g o`, `g s`, `g n`, `g p`, `g m` | the overview, servers, networks, players and nodes     |
| `g t`, `g f`, `g e`               | templates, file sets, and plugins and mods             |
| `g b`, `g c`                      | backup jobs and schedules                              |
| `g l`, `g ,`, `g a`               | the log, the settings and the user's account           |
| `c s`, `c n`, `c m`               | **Create server**, **Create network** and **Add node** |

## Operations

Long actions run as **operations**: creating, copying and changing servers, stopping and restarting them, updating their
image, installing plugins on many servers, backing up and restoring, and the actions on networks and on many servers at
once. The panel shows their steps as they go, e.g. how much of a server image is downloaded or how many servers of a
network are configured. A dialog can't be closed by mistake meanwhile; **Continue in the background** hands the
operation to a notification, which follows it to its end and links to its result. The operations of the last hour, also
those of other users, are in the list behind the button next to the warnings. The master runs them in the background:
an answer comes right away if the action ends within a second, otherwise `202 Accepted` with the operation, which
`GET /api/operations/{id}` follows, so that neither a closed browser nor a proxy in front of the master cuts it off. The
master can't restart while one runs. Agents tell the progress of their part, e.g. the bytes of a download, through
`ProgressService`; agents of older versions only let the panel show the steps.

Operations whose steps can stop safely show **Cancel** in their dialog, notification and list: creating and copying
servers, backing up, installing plugins, applying file sets and the actions on players and on many servers at once
(`POST /api/operations/{id}/cancel`). The operation stops at its next step that can stop, calls to the agents stop with
it, and it ends as **cancelled**; one that was done before it noticed ends as done. A cancelled creation or copy of a
server leaves no server; once a new server has its modpack, it stays, as installing its plugins can't be cancelled. An
action on many servers begins no more servers, but finishes on those it began, so that e.g. no restart is cut short and
leaves a server stopped. Applying a file set can be cancelled while it writes the files, but not once it restarts
servers. Restoring backups, moving servers, stopping and restarting them, changing their settings or image, and the
actions on networks and datastores can't be cancelled, as they must finish once they began. Whoever started an
operation may cancel it, and so may others who may do the same, e.g. back up the server; for a file set, those who may
change the files of all servers.

Actions on many servers tell how they ended on each: installing plugins, applying a file set, and the actions on players
and on many servers at once. **Retry the failed ones** tries the same again on the servers where it failed.

## Languages

The panel speaks English and German. It follows the browser until someone chooses a language in the menu of their name
or on their account page, which the panel stores for the signed-in user, so that it applies in all their browsers; on
the sign-in page, the button next to the colour theme chooses it for the browser. Dates, times and numbers follow the
language too. What the master and the agents send, such as errors, the log and the descriptions of permissions, stays
English.

## Keyboard and screen readers

Each page names itself in the title of the browser's tab, e.g. `Files · lobby · Servers · Noryx`. The first press of
Tab shows **Skip to content**, which jumps past the navigation. Opening another page moves the focus to its heading, so
that screen readers announce it; switching the tabs of a page or changing its search leaves the focus where it is.
Choices in a row, such as the time range of charts, the view and state filter of servers or the colour theme, are one
stop of Tab at the chosen option, and the arrow keys, Home and End choose another. On touch screens, small icon buttons,
e.g. the pin of a server, grow to 40 px.
