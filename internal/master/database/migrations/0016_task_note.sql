-- last_note tells what the latest run of a task left out, e.g. servers without data to back up.
ALTER TABLE tasks ADD COLUMN last_note TEXT;
