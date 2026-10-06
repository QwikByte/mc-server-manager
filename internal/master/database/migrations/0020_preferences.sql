-- The layout of each user's overview: its widgets in their order as JSON, each with its width
-- in columns and whether it is hidden. Users without one see the panel's default layout.
CREATE TABLE dashboards (
    user_id INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    widgets TEXT    NOT NULL
);

-- The servers each user pinned in the panel, in the order of position. Pins go with the
-- user and with the node; the panel hides those of servers the user doesn't see.
CREATE TABLE pinned_servers (
    user_id   INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    position  INTEGER NOT NULL,
    node_id   TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT    NOT NULL,
    PRIMARY KEY (user_id, node_id, server_id)
)
