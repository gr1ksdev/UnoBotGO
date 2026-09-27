CREATE TABLE ranking_imports (
 import_id text PRIMARY KEY,
 chat_id bigint NOT NULL REFERENCES group_configs(chat_id),
 created_by bigint NOT NULL CHECK(created_by>0),
 created_at timestamptz NOT NULL,
 source_hash text NOT NULL,
 source_system text NOT NULL DEFAULT 'legacy' CHECK(source_system='legacy'),
 raw_text text NOT NULL,
 status text NOT NULL DEFAULT 'staged' CHECK(status IN ('staged','applied')),
 applied_at timestamptz,
 UNIQUE(chat_id,source_hash),
 CHECK((status='applied')=(applied_at IS NOT NULL))
);
CREATE TABLE ranking_import_entries (
 entry_id text PRIMARY KEY,
 import_id text NOT NULL REFERENCES ranking_imports(import_id),
 line_number integer NOT NULL CHECK(line_number>0),
 imported_name text NOT NULL,
 source_score_units bigint NOT NULL CHECK(source_score_units>=0),
 linked_user_id bigint CHECK(linked_user_id>0),
 status text NOT NULL CHECK(status IN ('unresolved','linked','ambiguous','invalid')),
 parse_error text,
 UNIQUE(import_id,line_number),
 CHECK((status='linked')=(linked_user_id IS NOT NULL))
);
-- Names are not unique. There is intentionally no import application/conversion.
