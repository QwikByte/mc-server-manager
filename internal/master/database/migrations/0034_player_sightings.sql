-- Where players were online, from the names of the players of game servers that the agents measure every minute: a
-- row per player, server and day (days since 1970 in UTC), with when the player was first and last seen that day and
-- for how many minutes. Names match regardless of case, as in Minecraft. Like the log, rows are kept for its retention.
CREATE TABLE player_sightings (
    name       TEXT    NOT NULL COLLATE NOCASE,
    node_id    TEXT    NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id  TEXT    NOT NULL,
    day        INTEGER NOT NULL,
    first_seen INTEGER NOT NULL,
    last_seen  INTEGER NOT NULL,
    minutes    INTEGER NOT NULL,
    PRIMARY KEY (name, node_id, server_id, day)
) WITHOUT ROWID;
CREATE INDEX player_sightings_node ON player_sightings (node_id, day);
CREATE INDEX player_sightings_day ON player_sightings (day)
