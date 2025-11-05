-- +goose Up
-- Add email_account_id to campaigns table to replace smtp_id
ALTER TABLE campaigns ADD COLUMN email_account_id BIGINT;

-- Add foreign key constraint to email_accounts table
ALTER TABLE campaigns ADD CONSTRAINT fk_campaigns_email_account
    FOREIGN KEY (email_account_id) REFERENCES email_accounts(id)
    ON DELETE SET NULL;

-- Add index for better query performance
CREATE INDEX idx_campaigns_email_account_id ON campaigns(email_account_id);

-- +goose Down
-- Remove the foreign key constraint
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS fk_campaigns_email_account;

-- Remove the index
DROP INDEX IF EXISTS idx_campaigns_email_account_id;

-- Remove the column
ALTER TABLE campaigns DROP COLUMN IF EXISTS email_account_id;
