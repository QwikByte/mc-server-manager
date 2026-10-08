-- Workflows start their steps when their triggers fire. definition holds their triggers, inputs,
-- steps and options as JSON, with the values of secret headers of requests, which the API never
-- returns. hook is the SHA-256 of the token in the URL that starts a workflow, empty for none.
-- saved_by is the user who saved a workflow last, whose permissions its runs need.
CREATE TABLE workflows (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL,
    definition  TEXT    NOT NULL,
    hook        TEXT    NOT NULL DEFAULT '',
    saved_by    INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
-- The runs of workflows, of which the newest 100 per workflow are kept: what started them, when
-- (in milliseconds), how they ended, and the steps they ran and the data of their trigger as JSON.
CREATE TABLE workflow_runs (
    id          INTEGER PRIMARY KEY,
    workflow_id TEXT    NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    trigger     TEXT    NOT NULL,
    started_by  TEXT    NOT NULL,
    started_at  INTEGER NOT NULL,
    ended_at    INTEGER NOT NULL,
    outcome     TEXT    NOT NULL,
    error       TEXT    NOT NULL,
    steps       TEXT    NOT NULL DEFAULT '[]',
    data        TEXT    NOT NULL DEFAULT '{}'
);
CREATE INDEX workflow_runs_workflow ON workflow_runs (workflow_id, id)
