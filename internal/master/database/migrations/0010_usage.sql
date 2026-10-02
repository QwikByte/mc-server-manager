-- What nodes and their running servers used, measured every minute and kept for a week.
-- An empty server_id stands for the node itself. cpu_millis is in thousandths of a core,
-- the network columns in bytes per second; players and tps are NULL if unknown.
CREATE TABLE usage_samples (
    time         INTEGER NOT NULL,
    node_id      TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id    TEXT    NOT NULL,
    cpu_millis   INTEGER NOT NULL,
    memory_bytes INTEGER NOT NULL,
    net_received INTEGER NOT NULL,
    net_sent     INTEGER NOT NULL,
    disk_bytes   INTEGER NOT NULL,
    players      INTEGER,
    tps          REAL
);
CREATE INDEX usage_samples_target ON usage_samples (node_id, server_id, time);
CREATE INDEX usage_samples_time ON usage_samples (time)
