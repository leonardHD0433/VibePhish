-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE "imap" (
    "user_id" BIGINT,
    "host" VARCHAR(255),
    "port" INTEGER,
    "username" VARCHAR(255),
    "password" VARCHAR(255),
    "modified_date" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "tls" BOOLEAN,
    "enabled" BOOLEAN,
    "folder" VARCHAR(255),
    "restrict_domain" VARCHAR(255),
    "delete_reported_campaign_email" BOOLEAN,
    "last_login" TIMESTAMP,
    "imap_freq" INTEGER
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS "imap";