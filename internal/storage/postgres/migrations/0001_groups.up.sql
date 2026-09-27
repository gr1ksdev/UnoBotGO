-- Only durable group metadata. Active games/decks/hands never belong here.
CREATE TABLE group_configs (
 chat_id bigint PRIMARY KEY,
 default_game_mode text NOT NULL DEFAULT 'classic' CHECK (default_game_mode IN ('classic','caseiro')),
 ranking_system text NOT NULL DEFAULT 'legacy' CHECK (ranking_system IN ('legacy','updated')),
 installed_by_user_id bigint,
 installed_at timestamptz,
 installation_update_id bigint,
 config_revision bigint NOT NULL DEFAULT 1 CHECK (config_revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
