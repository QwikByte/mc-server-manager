-- Notes of servers, e.g. what a test server is for or whom to ask about it. Like their tags,
-- the master keeps them by node and server.
CREATE TABLE server_notes (
    node_id   TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT NOT NULL,
    notes     TEXT NOT NULL,
    PRIMARY KEY (node_id, server_id)
)
