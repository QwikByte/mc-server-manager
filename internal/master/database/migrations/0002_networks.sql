-- A network is a Velocity proxy with game servers behind it. forwarding_secret lets the
-- backends verify the players the proxy forwards.
CREATE TABLE networks (
    id                TEXT    PRIMARY KEY,
    name              TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    proxy_node_id     TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    proxy_server_id   TEXT    NOT NULL,
    forwarding_secret TEXT    NOT NULL,
    created_at        INTEGER NOT NULL,
    UNIQUE (proxy_node_id, proxy_server_id)
);

-- Players join the backend at position 0. A server belongs to one network at most.
CREATE TABLE network_backends (
    network_id TEXT    NOT NULL REFERENCES networks (id) ON DELETE CASCADE,
    node_id    TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id  TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    position   INTEGER NOT NULL,
    PRIMARY KEY (network_id, name),
    UNIQUE (node_id, server_id)
);
