-- Change only the default for newly created group configurations.
-- Existing groups keep their ranking system, revision and scores.
ALTER TABLE group_configs ALTER COLUMN ranking_system SET DEFAULT 'updated';
