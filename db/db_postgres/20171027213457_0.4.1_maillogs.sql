-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE "mail_logs" (
    "id" SERIAL PRIMARY KEY,
    "campaign_id" INTEGER,
    "user_id" INTEGER,
    "send_date" TIMESTAMP,
    "send_attempt" INTEGER,
    "r_id" VARCHAR(255),
    "processing" BOOLEAN
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS "mail_logs";