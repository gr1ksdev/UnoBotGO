-- Extend the audit status for an evaluated result with fewer than two eligible
-- placements. Existing results, payload hashes and scores are not rewritten.
ALTER TABLE completed_games DROP CONSTRAINT completed_games_scoring_status_check;
ALTER TABLE completed_games ADD CONSTRAINT completed_games_scoring_status_check
 CHECK(scoring_status IN ('needs_product_decision','scored','insufficient_eligible_players'));
-- The original consistency check remains: no-award results have scored_at NULL.
