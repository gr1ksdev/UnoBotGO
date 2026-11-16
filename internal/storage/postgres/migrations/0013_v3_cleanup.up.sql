-- V3 cleanup: the product is a Telegram-only bot with a single house mode.
--
-- ranking_private and user_privacy_settings only affected the Mini App ranking,
-- which no longer exists. default_game_mode became meaningless once the house
-- rules became the only rules.
--
-- Historical rows are preserved; only the removed configuration surfaces are
-- dropped. The legacy room_type/player_individual_stats columns stay inert.

DROP TABLE IF EXISTS user_privacy_settings;

ALTER TABLE group_configs DROP COLUMN IF EXISTS ranking_private;
ALTER TABLE group_configs DROP COLUMN IF EXISTS default_game_mode;
