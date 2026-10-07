-- Thresholds of usage that nodes and servers have instead of the defaults of the master's settings,
-- as a JSON object by measure, e.g. {"cpu": {"value": 95, "minutes": 15}}; measures it leaves out
-- use the defaults. An empty server_id stands for the node itself. Like their usage, the master
-- keeps them by node and server.
CREATE TABLE usage_thresholds (
    node_id    TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id  TEXT NOT NULL,
    thresholds TEXT NOT NULL,
    PRIMARY KEY (node_id, server_id)
)
