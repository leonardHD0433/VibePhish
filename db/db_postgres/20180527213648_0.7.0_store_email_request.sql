-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE "email_requests" (
    "id" SERIAL PRIMARY KEY,
    "user_id" INTEGER,
    "template_id" INTEGER,
    "page_id" INTEGER,
    "first_name" VARCHAR(255),
    "last_name" VARCHAR(255),
    "email" VARCHAR(255),
    "position" VARCHAR(255),
    "url" VARCHAR(255),
    "r_id" VARCHAR(255),
    "from_address" VARCHAR(255)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS "email_requests";