-- Networks are set up the way their proxy documents it. proxy_type is velocity, bungeecord
-- or waterfall, as a network can't ask a proxy whose node is offline. forwarding is modern
-- (Velocity's) or legacy (BungeeCord's ip_forward); firewalled records that the operator
-- confirmed that only the proxy's node reaches the backends on other nodes, which legacy
-- forwarding needs. try lists the names of the backends players join and fall back to, in
-- this order; forced_hosts the backends of host names, as [{"host": …, "servers": […]}].
ALTER TABLE networks ADD COLUMN proxy_type TEXT NOT NULL DEFAULT 'velocity';
ALTER TABLE networks ADD COLUMN forwarding TEXT NOT NULL DEFAULT 'modern';
ALTER TABLE networks ADD COLUMN firewalled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE networks ADD COLUMN try TEXT NOT NULL DEFAULT '[]';
ALTER TABLE networks ADD COLUMN forced_hosts TEXT NOT NULL DEFAULT '[]';

-- Settings of BungeeCord: only players with the permission bungeecord.server.<name> may join a
-- restricted server, and the MOTD is shown for host names that lead to it.
ALTER TABLE network_backends ADD COLUMN restricted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE network_backends ADD COLUMN motd TEXT NOT NULL DEFAULT '';

-- Players joined the first server of a network so far.
UPDATE networks SET try = COALESCE(
    (SELECT json_array(name) FROM network_backends b WHERE b.network_id = networks.id AND b.position = 0), '[]');
