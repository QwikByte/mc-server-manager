-- Channels send notifications to Discord, Slack, a webhook or by mail. secret is the URL of a
-- webhook or the password of the mail server, which the API never returns; email holds the
-- other settings of a mail channel as JSON.
CREATE TABLE notification_channels (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    kind       TEXT    NOT NULL,
    email      TEXT    NOT NULL DEFAULT '',
    secret     TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);
-- Rules send the new log entries of at least a level to a channel: of the categories in the JSON
-- array categories (all if it is empty), and of a node or server if they are set.
CREATE TABLE notification_rules (
    id         TEXT    PRIMARY KEY,
    channel_id TEXT    NOT NULL REFERENCES notification_channels (id) ON DELETE CASCADE,
    enabled    INTEGER NOT NULL,
    level      TEXT    NOT NULL,
    categories TEXT    NOT NULL,
    node_id    TEXT    NOT NULL DEFAULT '',
    server_id  TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
)
