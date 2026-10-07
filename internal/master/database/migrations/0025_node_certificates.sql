-- When the certificate of a node expires, so that the panel warns about a node that stays offline
-- until it has to be enrolled again, also after the master restarted. The master stores it as it
-- issues a certificate, and when a node presents one that expires later; nodes enrolled before get
-- it once they are online.
ALTER TABLE nodes ADD COLUMN certificate_expires_at INTEGER
