-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Change default value of smtp_encryption from 'tls' to 'none'
ALTER TABLE email_accounts
ALTER COLUMN smtp_encryption SET DEFAULT 'none';

-- Update existing records that might have NULL encryption to 'none'
UPDATE email_accounts
SET smtp_encryption = 'none'
WHERE smtp_encryption IS NULL;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Revert default value back to 'tls'
ALTER TABLE email_accounts
ALTER COLUMN smtp_encryption SET DEFAULT 'tls';
