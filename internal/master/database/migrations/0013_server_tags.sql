-- Tags of servers, e.g. lobby or bedwars, by which the panel finds and groups them. Servers
-- live on their agents, so the master keeps their tags by node and server.
CREATE TABLE server_tags (
    node_id   TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT NOT NULL,
    tag       TEXT NOT NULL,
    PRIMARY KEY (node_id, server_id, tag)
)
