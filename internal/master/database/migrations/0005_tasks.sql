-- Tasks run on a schedule: backup jobs and policies such as nightly restarts. kind tells
-- them apart; schedule and settings are JSON, see the schedule package and the kinds.
CREATE TABLE tasks (
    id          TEXT    PRIMARY KEY,
    kind        TEXT    NOT NULL,
    name        TEXT    NOT NULL COLLATE NOCASE,
    enabled     INTEGER NOT NULL,
    schedule    TEXT    NOT NULL,
    settings    TEXT    NOT NULL,
    last_run_at INTEGER,
    last_error  TEXT,
    created_at  INTEGER NOT NULL,
    UNIQUE (kind, name)
);

-- The servers a task runs on. An empty server_id stands for all servers of the node,
-- including those created later.
CREATE TABLE task_targets (
    task_id   TEXT NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    node_id   TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT NOT NULL,
    PRIMARY KEY (task_id, node_id, server_id)
)
