-- Settings for the servers of a node. Without a port range, any port can be used; without
-- a memory reserve, servers can get more memory than the node has.
ALTER TABLE nodes ADD COLUMN default_storage TEXT NOT NULL DEFAULT 'default';
ALTER TABLE nodes ADD COLUMN port_min INTEGER;
ALTER TABLE nodes ADD COLUMN port_max INTEGER;
ALTER TABLE nodes ADD COLUMN memory_reserve_mb INTEGER DEFAULT 1024;
