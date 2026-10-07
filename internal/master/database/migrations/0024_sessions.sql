-- Users see where they are signed in and end sessions. id names a session to the panel, but
-- unlike its token and the token's hash, it can't sign in. created_at is 0 for sessions from
-- before. last_used_at, ip, browser and os tell when and from where a session was used last,
-- noted at most once a minute; browser and os are names the master recognised in the
-- User-Agent, or empty.
ALTER TABLE sessions ADD COLUMN id TEXT NOT NULL DEFAULT '';
UPDATE sessions SET id = lower(hex(randomblob(16)));
CREATE UNIQUE INDEX sessions_id ON sessions (id);
ALTER TABLE sessions ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN last_used_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN ip TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN browser TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN os TEXT NOT NULL DEFAULT ''
