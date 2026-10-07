-- The size of the log, which the logs package keeps under the limit of the settings. size is about
-- the bytes an entry takes in the database: its texts, the IDs again in their index, and 64 bytes
-- for the numbers, the row and the index of the time. As it is computed, adding it rewrites nothing.
ALTER TABLE log_entries ADD COLUMN size INTEGER GENERATED ALWAYS AS (
    octet_length(source) + octet_length(category) + octet_length(message) + octet_length(username) +
    octet_length(node_name) + octet_length(server_name) + octet_length(attrs) +
    2 * (octet_length(node_id) + octet_length(server_id)) + 64
) VIRTUAL;

-- The number of entries and their size in all, kept up to date by the triggers, so that pruning
-- needn't count them. Entries are only added and deleted, never changed.
CREATE TABLE log_size (
    entries INTEGER NOT NULL,
    bytes   INTEGER NOT NULL
);
INSERT INTO log_size SELECT COUNT(*), COALESCE(SUM(size), 0) FROM log_entries;

CREATE TRIGGER log_entries_added AFTER INSERT ON log_entries BEGIN
    UPDATE log_size SET entries = entries + 1, bytes = bytes + NEW.size;
END;

CREATE TRIGGER log_entries_deleted AFTER DELETE ON log_entries BEGIN
    UPDATE log_size SET entries = entries - 1, bytes = bytes - OLD.size;
END
