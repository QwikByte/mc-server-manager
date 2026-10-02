-- The log of the master and its agents, see the logs package. time is in milliseconds,
-- level that of Go's log/slog: -4 debug, 0 info, 4 warn, 8 error. node_id and server_id
-- tell what an entry is about, which decides who may see it. Entries outlive their nodes
-- and servers, so they keep the names these had. attrs is a JSON object of strings.
CREATE TABLE log_entries (
    id          INTEGER PRIMARY KEY,
    time        INTEGER NOT NULL,
    level       INTEGER NOT NULL,
    source      TEXT    NOT NULL,
    category    TEXT    NOT NULL,
    message     TEXT    NOT NULL,
    username    TEXT    NOT NULL,
    node_id     TEXT    NOT NULL,
    node_name   TEXT    NOT NULL,
    server_id   TEXT    NOT NULL,
    server_name TEXT    NOT NULL,
    attrs       TEXT    NOT NULL
);
CREATE INDEX log_entries_time ON log_entries (time);
CREATE INDEX log_entries_target ON log_entries (node_id, server_id);

-- Where the master continues reading the log of a node's agent.
CREATE TABLE log_cursors (
    node_id TEXT    PRIMARY KEY REFERENCES nodes (id) ON DELETE CASCADE,
    boot    TEXT    NOT NULL,
    seq     INTEGER NOT NULL
)
