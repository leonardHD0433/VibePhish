-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Create email_accounts table for managing email senders
CREATE TABLE email_accounts (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE, -- Email address (primary key conceptually)
    type VARCHAR(50) NOT NULL, -- 'noreply', 'notification', 'forgetpassword', 'marketing', 'support'

    -- SMTP Configuration
    smtp_host VARCHAR(255) NOT NULL, -- smtp.gmail.com, smtp.office365.com
    smtp_port INTEGER DEFAULT 587,
    smtp_encryption VARCHAR(10) DEFAULT 'tls', -- 'tls', 'ssl', 'none'

    -- n8n Integration (Reference to n8n credential store)
    n8n_credential_id VARCHAR(100), -- Link to encrypted credentials in n8n
    n8n_credential_name VARCHAR(255), -- Display name in n8n

    -- Usage Tracking
    usage_count INTEGER DEFAULT 0,
    last_used TIMESTAMP,

    -- Status and Metadata
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_email_accounts_email ON email_accounts(email);
CREATE INDEX idx_email_accounts_type ON email_accounts(type, is_active);
CREATE INDEX idx_email_accounts_active ON email_accounts(is_active);

-- Create email sending logs for audit trail
CREATE TABLE email_sending_logs (
    id SERIAL PRIMARY KEY,
    email_account_id INTEGER REFERENCES email_accounts(id) ON DELETE SET NULL,
    recipient_email VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    status VARCHAR(50) NOT NULL, -- 'sent', 'failed', 'bounced'
    error_message TEXT,
    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    n8n_execution_id VARCHAR(100) -- Link to n8n execution for debugging
);

-- Create indexes for email logs
CREATE INDEX idx_email_logs_account ON email_sending_logs(email_account_id, sent_at);
CREATE INDEX idx_email_logs_status ON email_sending_logs(status);
CREATE INDEX idx_email_logs_recipient ON email_sending_logs(recipient_email);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_email_logs_recipient;
DROP INDEX IF EXISTS idx_email_logs_status;
DROP INDEX IF EXISTS idx_email_logs_account;
DROP TABLE IF EXISTS email_sending_logs;
DROP INDEX IF EXISTS idx_email_accounts_active;
DROP INDEX IF EXISTS idx_email_accounts_type;
DROP INDEX IF EXISTS idx_email_accounts_email;
DROP TABLE IF EXISTS email_accounts;
