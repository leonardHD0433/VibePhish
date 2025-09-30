-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Move the relationship between campaigns and smtp to campaigns
ALTER TABLE campaigns ADD COLUMN "smtp_id" BIGINT;
-- Create a new table to store smtp records
DROP TABLE IF EXISTS smtp;
CREATE TABLE smtp(
    id SERIAL PRIMARY KEY,
    user_id BIGINT,
    interface_type VARCHAR(255),
    name VARCHAR(255),
    host VARCHAR(255),
    username VARCHAR(255),
    password VARCHAR(255),
    from_address VARCHAR(255),
    modified_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ignore_cert_errors BOOLEAN
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS smtp;
ALTER TABLE campaigns DROP COLUMN IF EXISTS smtp_id;