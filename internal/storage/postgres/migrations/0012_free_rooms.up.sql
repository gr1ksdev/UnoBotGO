ALTER TABLE completed_games ALTER COLUMN chat_id DROP NOT NULL;
ALTER TABLE completed_games ADD COLUMN room_type text NOT NULL DEFAULT 'group' CHECK (room_type IN ('group','free'));
ALTER TABLE completed_games ADD CONSTRAINT completed_games_room_chat_check
 CHECK ((room_type='group' AND chat_id IS NOT NULL) OR (room_type='free' AND chat_id IS NULL));

CREATE TABLE player_individual_stats (
 user_id bigint NOT NULL CHECK(user_id>0),
 ranking_system text NOT NULL CHECK(ranking_system IN ('legacy','updated')),
 score_units bigint NOT NULL CHECK(score_units>=0),
 completed_games bigint NOT NULL CHECK(completed_games>0),
 wins bigint NOT NULL CHECK(wins>=0),
 display_name text NOT NULL,
 last_finished_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,ranking_system)
);
