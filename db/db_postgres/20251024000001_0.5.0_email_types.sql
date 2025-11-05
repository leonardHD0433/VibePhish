-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Create email_types table for configurable email account types
CREATE TABLE email_types (
    id SERIAL PRIMARY KEY,
    value VARCHAR(50) NOT NULL UNIQUE, -- 'noreply', 'notification', etc.
    display_name VARCHAR(100) NOT NULL, -- 'No Reply', 'Notification', etc.
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0, -- For ordering in dropdown
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for active types
CREATE INDEX idx_email_types_active ON email_types(is_active, sort_order);

-- Insert default email types
INSERT INTO email_types (value, display_name, description, sort_order) VALUES
    ('noreply', 'No Reply', 'System notifications users should not reply to', 1),
    ('notification', 'Notification', 'General alerts and updates', 2),
    ('forgetpassword', 'Forget Password', 'Password reset emails', 3),
    ('marketing', 'Marketing', 'Promotional campaigns and newsletters', 4),
    ('support', 'Support', 'Customer support communications', 5);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_email_types_active;
DROP TABLE IF EXISTS email_types;
