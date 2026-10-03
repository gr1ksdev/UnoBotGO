-- Change only the default for newly created group configurations.
-- Existing groups keep their configured mode, revision and scores.
ALTER TABLE group_configs ALTER COLUMN default_game_mode SET DEFAULT 'caseiro';
