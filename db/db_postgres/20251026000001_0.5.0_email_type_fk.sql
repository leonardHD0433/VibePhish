-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Rename column from 'type' to 'email_type' for clarity
ALTER TABLE email_accounts
RENAME COLUMN type TO email_type;

-- Add foreign key constraint to email_types table
-- ON UPDATE CASCADE: If email_type value changes, update all accounts
-- ON DELETE RESTRICT: Prevent deleting email types that are in use
ALTER TABLE email_accounts
ADD CONSTRAINT fk_email_accounts_email_type
FOREIGN KEY (email_type)
REFERENCES email_types(value)
ON UPDATE CASCADE
ON DELETE RESTRICT;

-- Drop old index and create new one with updated column name
DROP INDEX IF EXISTS idx_email_accounts_type;
CREATE INDEX idx_email_accounts_email_type ON email_accounts(email_type, is_active);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Remove foreign key constraint
ALTER TABLE email_accounts
DROP CONSTRAINT IF EXISTS fk_email_accounts_email_type;

-- Drop new index and recreate old one
DROP INDEX IF EXISTS idx_email_accounts_email_type;
CREATE INDEX idx_email_accounts_type ON email_accounts(type, is_active);

-- Rename column back to 'type'
ALTER TABLE email_accounts
RENAME COLUMN email_type TO type;
