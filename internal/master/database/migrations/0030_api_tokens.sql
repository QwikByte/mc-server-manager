-- API tokens let scripts use the API as their user, with all of the user's permissions or only
-- some. Only the SHA-256 hash of a token is stored; id names it to the panel and in the log, but
-- unlike the token and its hash, it can't authenticate. permissions is a JSON array of the
-- permissions it may use, or NULL for all of the user's. expires_at is 0 for tokens that don't
-- expire. last_used_at and ip tell when and from where a token was used last, noted at most once
-- a minute.
CREATE TABLE api_tokens (
    token_hash   BLOB    PRIMARY KEY,
    id           TEXT    NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    permissions  TEXT,
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL DEFAULT 0,
    ip           TEXT    NOT NULL DEFAULT '',
    UNIQUE (user_id, name)
)
