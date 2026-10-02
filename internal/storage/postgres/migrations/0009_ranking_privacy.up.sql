ALTER TABLE group_configs
ADD COLUMN ranking_private boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS user_privacy_settings (
    user_id bigint PRIMARY KEY CHECK (user_id > 0),
    ranking_private boolean NOT NULL DEFAULT false
);
