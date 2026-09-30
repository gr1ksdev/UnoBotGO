CREATE TABLE IF NOT EXISTS player_group_monthly_stats (
 chat_id bigint NOT NULL REFERENCES group_configs(chat_id),
 user_id bigint NOT NULL CHECK(user_id>0),
 month_start date NOT NULL,
 ranking_system text NOT NULL CHECK(ranking_system IN ('legacy','updated')),
 score_units bigint NOT NULL CHECK(score_units>=0),
 completed_games bigint NOT NULL CHECK(completed_games>0),
 wins bigint NOT NULL CHECK(wins>=0),
 display_name text NOT NULL,
 last_finished_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(chat_id,user_id,month_start)
);

CREATE INDEX IF NOT EXISTS player_group_monthly_stats_ranking ON player_group_monthly_stats(chat_id,month_start,score_units DESC,user_id);

INSERT INTO player_group_monthly_stats(chat_id,user_id,month_start,ranking_system,score_units,completed_games,wins,display_name,last_finished_at)
SELECT
 g.chat_id,
 p.user_id,
 (date_trunc('month', (g.finished_at AT TIME ZONE 'America/Sao_Paulo')))::date AS month_start,
 g.ranking_system,
 sum(COALESCE(p.score_units, 0))::bigint AS score_units,
 count(*)::bigint AS completed_games,
 count(*) FILTER (WHERE p.position = 1)::bigint AS wins,
 (array_agg(p.observed_name ORDER BY g.finished_at DESC, g.game_id DESC))[1] AS display_name,
 max(g.finished_at) AS last_finished_at
FROM completed_games g
JOIN completed_game_players p ON p.game_id = g.game_id
WHERE g.scoring_status = 'scored'
  AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out))
GROUP BY g.chat_id, p.user_id, (date_trunc('month', (g.finished_at AT TIME ZONE 'America/Sao_Paulo')))::date, g.ranking_system
ON CONFLICT (chat_id, user_id, month_start) DO NOTHING;
