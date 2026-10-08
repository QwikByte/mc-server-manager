-- S3-compatible storage that backup jobs copy their backups to, with its credentials, which
-- the API never returns.
CREATE TABLE backup_storages (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    endpoint   TEXT    NOT NULL,
    region     TEXT    NOT NULL,
    bucket     TEXT    NOT NULL,
    prefix     TEXT    NOT NULL,
    access_key TEXT    NOT NULL,
    secret_key TEXT    NOT NULL,
    path_style INTEGER NOT NULL,
    encrypt    INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

-- Copies of backups that jobs made away from the servers' nodes: in a storage or on another
-- node (copy_node, in a storage location of it). They describe the backup and the server it
-- is of as they were when it was copied, so that the server can be restored once it or its
-- node is gone; deleting the job or the server keeps them. They belong to the server on its
-- node (node_id), which they follow when it moves. Copies of a deleted storage or on a removed
-- node go with it.
CREATE TABLE backup_copies (
    id          INTEGER PRIMARY KEY,
    task_id     TEXT    REFERENCES tasks (id) ON DELETE SET NULL,
    storage_id  TEXT    REFERENCES backup_storages (id) ON DELETE CASCADE,
    copy_node   TEXT    REFERENCES nodes (id) ON DELETE CASCADE,
    location    TEXT    NOT NULL,
    server_id   TEXT    NOT NULL,
    server_name TEXT    NOT NULL,
    proxy       INTEGER NOT NULL,
    node_id     TEXT    NOT NULL,
    node_name   TEXT    NOT NULL,
    backup_id   TEXT    NOT NULL,
    label       TEXT    NOT NULL,
    created_at  INTEGER NOT NULL,
    size        INTEGER NOT NULL,
    paths       TEXT    NOT NULL,
    exclude     TEXT    NOT NULL,
    kept        INTEGER NOT NULL,
    copied_at   INTEGER NOT NULL,
    CHECK ((storage_id IS NOT NULL) + (copy_node IS NOT NULL) = 1)
);
CREATE UNIQUE INDEX backup_copies_storage ON backup_copies (storage_id, server_id, backup_id) WHERE storage_id IS NOT NULL;
CREATE UNIQUE INDEX backup_copies_node ON backup_copies (copy_node, server_id, backup_id) WHERE copy_node IS NOT NULL;
CREATE INDEX backup_copies_server ON backup_copies (server_id)
