-- The language of the panel a user chose, e.g. de; empty follows the browser.
ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT ''
