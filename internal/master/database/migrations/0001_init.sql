CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    created_at    INTEGER NOT NULL
);

-- Only the SHA-256 hash of a session token is stored.
CREATE TABLE sessions (
    token_hash BLOB    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

-- join_secret_hash/join_expires_at hold the pending single-use join token of a node.
CREATE TABLE nodes (
    id               TEXT    PRIMARY KEY,
    name             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    address          TEXT    NOT NULL,
    join_secret_hash BLOB,
    join_expires_at  INTEGER,
    enrolled_at      INTEGER,
    created_at       INTEGER NOT NULL
)
