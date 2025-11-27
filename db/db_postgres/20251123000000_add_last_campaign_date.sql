-- +goose Up
-- +goose StatementBegin
-- Add last_campaign_date column to targets table to track cybersecurity fatigue
-- This column stores the date when a target was last included in a campaign
-- NULL means the target has never been included in a campaign
ALTER TABLE targets ADD COLUMN IF NOT EXISTS last_campaign_date TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- Add index for efficient filtering by last campaign date
-- Useful for queries like "find targets not contacted in the last X days"
CREATE INDEX IF NOT EXISTS idx_targets_last_campaign_date ON targets(last_campaign_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_targets_last_campaign_date;
ALTER TABLE targets DROP COLUMN IF EXISTS last_campaign_date;
-- +goose StatementEnd
