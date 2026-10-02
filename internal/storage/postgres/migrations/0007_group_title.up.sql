ALTER TABLE group_configs ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS player_group_monthly_stats_user_month ON player_group_monthly_stats(user_id, month_start);
