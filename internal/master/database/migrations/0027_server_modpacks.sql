-- The Modrinth modpack each server was created from, so that it can move to another version
-- of the pack. Like their tags, the master keeps it by node and server.
CREATE TABLE server_modpacks (
    node_id   TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    server_id TEXT NOT NULL,
    project   TEXT NOT NULL,
    -- The ID of the version on Modrinth, its version number, and the Minecraft and loader
    -- version it needs.
    version   TEXT NOT NULL,
    number    TEXT NOT NULL,
    minecraft TEXT NOT NULL,
    loader    TEXT NOT NULL,
    -- The files the pack wrote, as a JSON object by their path in the server's data: their
    -- SHA-512 hash and, for mods, their project on Modrinth.
    files     TEXT NOT NULL,
    PRIMARY KEY (node_id, server_id)
)
