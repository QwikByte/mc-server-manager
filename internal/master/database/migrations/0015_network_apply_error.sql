-- apply_error tells why the servers of a network were last configured in vain, until they
-- are configured again: its proxy may send players to where they no longer are.
ALTER TABLE networks ADD COLUMN apply_error TEXT;
