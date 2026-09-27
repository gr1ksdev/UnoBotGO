CREATE TABLE completed_games (
 game_id text PRIMARY KEY,
 chat_id bigint NOT NULL REFERENCES group_configs(chat_id),
 game_mode text NOT NULL CHECK(game_mode IN ('classic','caseiro')),
 ranking_system text NOT NULL CHECK(ranking_system IN ('legacy','updated')),
 config_revision bigint NOT NULL CHECK(config_revision>0),
 started_at timestamptz NOT NULL,
 finished_at timestamptz NOT NULL CHECK(finished_at>=started_at),
 final_revision bigint NOT NULL CHECK(final_revision>0),
 finish_reason text NOT NULL CHECK(finish_reason IN ('completed','departure')),
 payload_hash text NOT NULL,
 participant_count integer NOT NULL CHECK(participant_count>0),
 scoring_status text NOT NULL CHECK(scoring_status IN ('needs_product_decision','scored')),
 policy_version text,
 scored_at timestamptz,
 CHECK((scoring_status='scored')=(policy_version IS NOT NULL AND scored_at IS NOT NULL))
);
CREATE TABLE completed_game_players (
 game_id text NOT NULL REFERENCES completed_games(game_id),
 user_id bigint NOT NULL CHECK(user_id>0),
 observed_name text NOT NULL,
 username text,
 final_status text NOT NULL CHECK(final_status IN ('playing','went_out','left')),
 position integer CHECK(position>0),
 went_out boolean NOT NULL,
 score_units bigint CHECK(score_units>=0),
 joined_after_start boolean NOT NULL,
 leave_count integer NOT NULL CHECK(leave_count>=0),
 reentry_count integer NOT NULL CHECK(reentry_count>=0),
 PRIMARY KEY(game_id,user_id),
 UNIQUE(game_id,position)
);
CREATE TABLE player_group_stats (
 chat_id bigint NOT NULL REFERENCES group_configs(chat_id),
 user_id bigint NOT NULL CHECK(user_id>0),
 ranking_system text NOT NULL CHECK(ranking_system IN ('legacy','updated')),
 score_units bigint NOT NULL CHECK(score_units>=0),
 completed_games bigint NOT NULL CHECK(completed_games>0),
 wins bigint NOT NULL CHECK(wins>=0),
 display_name text NOT NULL,
 last_finished_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(chat_id,user_id)
);
CREATE INDEX player_group_stats_ranking ON player_group_stats(chat_id,score_units DESC,user_id);
