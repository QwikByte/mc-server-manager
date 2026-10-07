-- Plugins and mods that a server keeps at their version: updating all of them, on its own or
-- on many servers, and scheduled updates leave these projects out. Like the tags of servers,
-- the master keeps them by node and server.
CREATE TABLE plugin_pins (
    node_id    TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id  TEXT NOT NULL,
    project_id TEXT NOT NULL,
    PRIMARY KEY (node_id, server_id, project_id)
)
