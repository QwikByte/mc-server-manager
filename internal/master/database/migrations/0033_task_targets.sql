-- Tasks also target the servers of tags and networks, which follow them as they change. A
-- target is a server, all servers of a node (server_id empty), those with a tag, or those of
-- a network: its game servers, its proxy or both (role servers, proxy or empty). Targets of
-- deleted nodes and networks go with them.
CREATE TABLE task_targets_new (
    task_id    TEXT NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    node_id    TEXT REFERENCES nodes (id) ON DELETE CASCADE,
    server_id  TEXT NOT NULL DEFAULT '',
    tag        TEXT NOT NULL DEFAULT '',
    network_id TEXT REFERENCES networks (id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT '',
    CHECK ((node_id IS NOT NULL) + (tag != '') + (network_id IS NOT NULL) = 1)
);
INSERT INTO task_targets_new (task_id, node_id, server_id) SELECT task_id, node_id, server_id FROM task_targets ORDER BY rowid;
DROP TABLE task_targets;
ALTER TABLE task_targets_new RENAME TO task_targets;
CREATE INDEX task_targets_task ON task_targets (task_id)
