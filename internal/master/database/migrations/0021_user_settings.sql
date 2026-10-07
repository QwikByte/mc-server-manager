-- The other settings of the panel each user chose, as a JSON object of known keys and values,
-- e.g. {"theme":"dark","clock":"24h"}. Keys a user never set follow the browser.
CREATE TABLE user_settings (
    user_id  INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    settings TEXT    NOT NULL
)
