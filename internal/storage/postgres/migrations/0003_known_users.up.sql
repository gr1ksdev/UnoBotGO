CREATE TABLE known_group_users (
 chat_id bigint NOT NULL REFERENCES group_configs(chat_id),
 user_id bigint NOT NULL CHECK(user_id>0),
 display_name text NOT NULL,
 username text,
 last_seen_at timestamptz NOT NULL,
 PRIMARY KEY(chat_id,user_id)
);
