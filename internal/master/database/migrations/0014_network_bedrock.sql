-- Bedrock players join a network through Geyser on its proxy at bedrock_port over UDP; 0
-- lets none join.
ALTER TABLE networks ADD COLUMN bedrock_port INTEGER NOT NULL DEFAULT 0;
