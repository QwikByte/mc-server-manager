-- The private WireGuard network of the nodes: its private IPv4 range, which only changes
-- while no node is a member, and the UDP port and MTU of the nodes' interfaces.
CREATE TABLE overlay_settings (
    id     INTEGER PRIMARY KEY CHECK (id = 1),
    subnet TEXT    NOT NULL,
    port   INTEGER NOT NULL,
    mtu    INTEGER NOT NULL
);
INSERT INTO overlay_settings (id, subnet, port, mtu) VALUES (1, '10.213.0.0/24', 51820, 1420);

-- The members of the network, with their address in it and their public key; the private
-- keys stay on the nodes. endpoint is host:port at which the other nodes reach a member;
-- empty for the host of its agent's address and the network's port.
CREATE TABLE overlay_nodes (
    node_id    TEXT    PRIMARY KEY REFERENCES nodes (id) ON DELETE CASCADE,
    address    TEXT    NOT NULL UNIQUE,
    public_key TEXT    NOT NULL UNIQUE,
    endpoint   TEXT    NOT NULL DEFAULT '',
    joined_at  INTEGER NOT NULL
);
