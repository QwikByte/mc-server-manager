-- Settings of the master that administrators change in the panel, as JSON, see the
-- settings package. There is at most one row; without it the defaults apply.
CREATE TABLE settings (
    id    INTEGER PRIMARY KEY CHECK (id = 1),
    value TEXT    NOT NULL
)
