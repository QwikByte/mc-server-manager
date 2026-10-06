# The panel

How the admin panel is organised, and what works the same on all of its pages.

## Overview

The **Overview** is the panel's start page: the players online, the servers by state, the nodes with what they use, the
networks, the servers with the most players, and what needs attention: crashing servers, offline nodes, nodes with more
memory assigned than they can give or almost full storage, and proxies that are stopped while their servers run. It
counts like the **Nodes** page: servers that run, not those that start or crash, and the memory assigned against what
the online nodes can give their servers, after the reserve. Sizes are in binary units (MiB, GiB).

The Overview is made of widgets: key figures, what needs attention, the nodes with their CPU of the last 24 hours, the
load of all nodes over the last 24 hours, pinned servers, networks, the servers with the most players, the latest
entries of the log and quick actions to create a server or a network or add a node. Each user only gets the widgets
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

## Search

**Ctrl+K** (⌘K) searches servers, also by tag, networks, nodes and pages from anywhere in the panel. A search that
starts with an action, e.g. `restart lobby`, starts, restarts or stops a server or opens its console.

## Operations

Long actions run as **operations**: creating, copying and changing servers, updating their image, installing plugins on
many servers, backing up and restoring, and the actions on networks and on many servers at once. The panel shows their
steps as they go, e.g. how much of a server image is downloaded or how many servers of a network are configured. A
dialog can't be closed by mistake meanwhile; **Continue in the background** hands the operation to a notification, which
follows it to its end and links to its result. The operations of the last hour, also those of other users, are in the
list behind the button next to the warnings. The master runs them in the background: an answer comes right away if the
action ends within a second, otherwise `202 Accepted` with the operation, which `GET /api/operations/{id}` follows, so
that neither a closed browser nor a proxy in front of the master cuts it off. The master can't restart while one runs.
Agents tell the progress of their part, e.g. the bytes of a download, through `ProgressService`; agents of older
versions only let the panel show the steps.

## Languages

The panel speaks English and German. It follows the browser until someone chooses a language in the menu of their name,
which the panel stores for the signed-in user, so that it applies in all their browsers; on the sign-in page, the button
next to the colour theme chooses it for the browser. Dates, times and numbers follow the language too. What the master
and the agents send, such as errors, the log and the descriptions of permissions, stays English.
