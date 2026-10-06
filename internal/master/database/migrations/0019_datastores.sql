-- Datastores: MariaDB and PostgreSQL servers of networks, which the agent of a node runs.
-- port is published at the node's address in the private network of the nodes for the
-- network's servers on other nodes; 0 until one needs it. Deleting a network through the
-- API needs its datastores deleted first; removing a node forgets them with its networks.
CREATE TABLE datastores (
    id         TEXT    PRIMARY KEY,
    network_id TEXT    NOT NULL REFERENCES networks (id) ON DELETE CASCADE,
    node_id    TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    engine     TEXT    NOT NULL,
    version    TEXT    NOT NULL,
    memory_mb  INTEGER NOT NULL,
    cpu_millis INTEGER NOT NULL,
    storage    TEXT    NOT NULL,
    port       INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    UNIQUE (network_id, name)
);

-- The databases of a datastore, each with a user of the same name. The API never returns
-- the passwords, which only reach the servers through file sets.
CREATE TABLE datastore_databases (
    datastore_id TEXT    NOT NULL REFERENCES datastores (id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    password     TEXT    NOT NULL,
    created_at   INTEGER NOT NULL,
    PRIMARY KEY (datastore_id, name)
);
