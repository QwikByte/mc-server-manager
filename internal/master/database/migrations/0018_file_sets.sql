-- File sets: named, versioned collections of text files that the master puts on the servers
-- of tags and networks. version is the newest of the versions, of which the newest 20 are
-- kept with their files as JSON, see the fileset package.
CREATE TABLE file_sets (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL,
    version     INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE file_set_versions (
    set_id     TEXT    NOT NULL REFERENCES file_sets (id) ON DELETE CASCADE,
    version    INTEGER NOT NULL,
    files      TEXT    NOT NULL,
    username   TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (set_id, version)
);

-- The servers a set is for: those with a tag, or the game servers or the proxy of a network.
CREATE TABLE file_set_targets (
    set_id TEXT NOT NULL REFERENCES file_sets (id) ON DELETE CASCADE,
    kind   TEXT NOT NULL,
    value  TEXT NOT NULL,
    role   TEXT NOT NULL,
    PRIMARY KEY (set_id, kind, value, role)
);

-- The secrets of a set, which only the agents fill in. The API only tells their names.
CREATE TABLE file_set_secrets (
    set_id     TEXT    NOT NULL REFERENCES file_sets (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (set_id, name)
)
