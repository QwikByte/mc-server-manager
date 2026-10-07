-- The variables of file sets with their values for servers, tags, networks and all servers, as
-- JSON, see the fileset package. Like the targets, they aren't part of the versions.
ALTER TABLE file_sets ADD COLUMN variables TEXT NOT NULL DEFAULT '[]'
