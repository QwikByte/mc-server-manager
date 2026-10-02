-- Disabled users can't sign in. Invited users have an empty password_hash until they set
-- a password with their setup link.
ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;

-- A setup link lets a user set a password once, for new users and resets. Only the SHA-256
-- hash of its token is stored.
CREATE TABLE setup_tokens (
    token_hash BLOB    PRIMARY KEY,
    user_id    INTEGER NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

-- Groups give their members permissions, see the access package. permissions is a JSON
-- array. Node and server permissions apply to all servers, or only to the targets of the
-- group if all_servers is 0.
CREATE TABLE user_groups (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL,
    permissions TEXT    NOT NULL,
    all_servers INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

-- An empty server_id stands for all servers of the node, including those created later.
CREATE TABLE group_targets (
    group_id  TEXT NOT NULL REFERENCES user_groups (id) ON DELETE CASCADE,
    node_id   TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT NOT NULL,
    PRIMARY KEY (group_id, node_id, server_id)
);

CREATE TABLE group_members (
    group_id TEXT    NOT NULL REFERENCES user_groups (id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

-- Administrators have every permission. All existing users were administrators.
INSERT INTO user_groups (id, name, description, permissions, all_servers, created_at)
VALUES ('administrators', 'Administrators', 'Have every permission, also those added in later versions.', '[]', 1, unixepoch());
INSERT INTO group_members (group_id, user_id) SELECT 'administrators', id FROM users
