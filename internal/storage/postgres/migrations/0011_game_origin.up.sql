ALTER TABLE completed_games ADD COLUMN origin text NOT NULL DEFAULT 'inline' CHECK (origin IN ('inline','webapp'));
CREATE INDEX completed_game_players_user_history ON completed_game_players(user_id, game_id);
