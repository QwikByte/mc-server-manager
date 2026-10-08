-- What the running datastores used, measured every minute and kept for a week like the usage of
-- servers. cpu_millis is in thousandths of a core; connections is NULL while a datastore
-- couldn't tell, e.g. as it started.
CREATE TABLE datastore_usage (
    time         INTEGER NOT NULL,
    datastore_id TEXT    NOT NULL REFERENCES datastores (id) ON DELETE CASCADE,
    cpu_millis   INTEGER NOT NULL,
    memory_bytes INTEGER NOT NULL,
    connections  INTEGER,
    disk_bytes   INTEGER NOT NULL
);
CREATE INDEX datastore_usage_target ON datastore_usage (datastore_id, time);
CREATE INDEX datastore_usage_time ON datastore_usage (time)
