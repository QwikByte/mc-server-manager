-- Templates for new servers. settings holds the settings, server.properties and plugins
-- as JSON, see the template package.
CREATE TABLE templates (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL,
    settings    TEXT    NOT NULL,
    created_at  INTEGER NOT NULL
)
