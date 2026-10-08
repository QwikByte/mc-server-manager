-- Which new warnings and errors of the log pop up for each user in the panel, as a JSON object:
-- the least level, whether only those of pinned servers and of chosen nodes, servers and
-- categories, and until when none at all. Users without one get all warnings and errors.
CREATE TABLE alert_filters (
    user_id INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    filter  TEXT    NOT NULL
)
