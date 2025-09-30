-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE headers(
    id SERIAL PRIMARY KEY,
    key VARCHAR(255),
    value VARCHAR(255),
    "smtp_id" BIGINT
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS headers;