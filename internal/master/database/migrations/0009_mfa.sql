-- Two-factor authentication: once enabled is 1, signing in also needs a code of the user's
-- authenticator app (TOTP); until then, secret belongs to a setup in progress. last_step is
-- the time step of the last code accepted, so that no code works twice. failures counts
-- wrong codes in a row, which lock the code check until locked_until.
CREATE TABLE user_mfa (
    user_id      INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret       TEXT    NOT NULL,
    enabled      INTEGER NOT NULL DEFAULT 0,
    last_step    INTEGER NOT NULL DEFAULT 0,
    failures     INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER NOT NULL DEFAULT 0
);

-- A recovery code replaces a code of the app once. Only its SHA-256 hash is stored.
CREATE TABLE recovery_codes (
    user_id   INTEGER NOT NULL REFERENCES user_mfa (user_id) ON DELETE CASCADE,
    code_hash BLOB    NOT NULL,
    PRIMARY KEY (user_id, code_hash)
)
