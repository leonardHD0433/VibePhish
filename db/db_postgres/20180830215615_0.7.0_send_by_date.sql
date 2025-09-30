-- +goose Up
-- SQL in this section is executed when the migration is applied.
ALTER TABLE campaigns ADD COLUMN send_by_date TIMESTAMP;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
ALTER TABLE campaigns DROP COLUMN IF EXISTS send_by_date;