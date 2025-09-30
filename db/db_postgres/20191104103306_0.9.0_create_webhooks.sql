-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE "webhooks" (
    "id" SERIAL PRIMARY KEY,
    "name" VARCHAR(255),
    "url" VARCHAR(1000),
    "secret" VARCHAR(255),
    "is_active" BOOLEAN DEFAULT FALSE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS "webhooks";