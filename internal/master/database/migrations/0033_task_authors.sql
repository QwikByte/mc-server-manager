-- The user who saved a task last. Steps of schedules that need more than the permission to
-- manage them, e.g. backing up servers first, run with this user's permissions, and not at
-- all once the user is deleted.
ALTER TABLE tasks ADD COLUMN saved_by INTEGER REFERENCES users (id) ON DELETE SET NULL
