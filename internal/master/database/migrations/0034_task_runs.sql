-- The runs of tasks, of which the newest 50 per task are kept, instead of only the latest:
-- when they started and ended, who started them by hand (empty for their schedule), their
-- outcome (succeeded or failed), error and note, and the steps of runs that take several, as
-- JSON.
CREATE TABLE task_runs (
    id         INTEGER PRIMARY KEY,
    task_id    TEXT    NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    started_at INTEGER NOT NULL,
    ended_at   INTEGER NOT NULL,
    started_by TEXT    NOT NULL,
    outcome    TEXT    NOT NULL,
    error      TEXT    NOT NULL,
    note       TEXT    NOT NULL,
    steps      TEXT    NOT NULL DEFAULT '[]'
);
CREATE INDEX task_runs_task ON task_runs (task_id, id);
INSERT INTO task_runs (task_id, started_at, ended_at, started_by, outcome, error, note)
SELECT id, last_run_at, last_run_at, '', CASE WHEN coalesce(last_error, '') = '' THEN 'succeeded' ELSE 'failed' END,
       coalesce(last_error, ''), coalesce(last_note, '')
FROM tasks WHERE last_run_at IS NOT NULL ORDER BY last_run_at;
ALTER TABLE tasks DROP COLUMN last_run_at;
ALTER TABLE tasks DROP COLUMN last_error;
ALTER TABLE tasks DROP COLUMN last_note
