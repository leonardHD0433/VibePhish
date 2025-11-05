-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Remove SMTP-related columns as OAuth2 doesn't need them
-- Only email, type, n8n credential info, and active status are needed
ALTER TABLE email_accounts
DROP COLUMN IF EXISTS smtp_host,
DROP COLUMN IF EXISTS smtp_port,
DROP COLUMN IF EXISTS smtp_encryption;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Restore SMTP columns if migration is rolled back
ALTER TABLE email_accounts
ADD COLUMN smtp_host VARCHAR(255),
ADD COLUMN smtp_port INT DEFAULT 587,
ADD COLUMN smtp_encryption VARCHAR(50) DEFAULT 'none';
