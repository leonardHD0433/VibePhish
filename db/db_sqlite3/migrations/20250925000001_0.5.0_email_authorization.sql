-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Create authorized_emails table for email-based access control
CREATE TABLE authorized_emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    normalized_email VARCHAR(255) NOT NULL UNIQUE, -- lowercase, trimmed for lookups
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, suspended, revoked
    role_id INTEGER REFERENCES roles(id),
    default_role VARCHAR(50) DEFAULT 'user', -- fallback role name
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME, -- optional expiration
    last_used_at DATETIME, -- track usage
    notes TEXT -- admin notes
);

-- Create indexes for performance
CREATE INDEX idx_authorized_emails_normalized ON authorized_emails(normalized_email, status);
CREATE INDEX idx_authorized_emails_status ON authorized_emails(status);
CREATE INDEX idx_authorized_emails_expires ON authorized_emails(expires_at) WHERE expires_at IS NOT NULL;

-- Create email authorization audit log
CREATE TABLE email_authorization_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(255) NOT NULL,
    normalized_email VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL, -- check, grant, deny, add, remove, suspend
    result VARCHAR(20) NOT NULL, -- success, denied, error
    ip_address VARCHAR(45), -- support IPv6
    user_agent TEXT,
    user_id INTEGER REFERENCES users(id), -- if authenticated user
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    details TEXT -- additional context
);

-- Create indexes for audit log queries
CREATE INDEX idx_email_auth_logs_email ON email_authorization_logs(normalized_email, created_at);
CREATE INDEX idx_email_auth_logs_created ON email_authorization_logs(created_at);
CREATE INDEX idx_email_auth_logs_action ON email_authorization_logs(action, result);

-- Create email domains table for domain-based authorization (optional)
CREATE TABLE authorized_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    default_role VARCHAR(50) DEFAULT 'user',
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

CREATE INDEX idx_authorized_domains_domain ON authorized_domains(domain, status);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_authorized_domains_domain;
DROP TABLE IF EXISTS authorized_domains;
DROP INDEX IF EXISTS idx_email_auth_logs_action;
DROP INDEX IF EXISTS idx_email_auth_logs_created;
DROP INDEX IF EXISTS idx_email_auth_logs_email;
DROP TABLE IF EXISTS email_authorization_logs;
DROP INDEX IF EXISTS idx_authorized_emails_expires;
DROP INDEX IF EXISTS idx_authorized_emails_status;
DROP INDEX IF EXISTS idx_authorized_emails_normalized;
DROP TABLE IF EXISTS authorized_emails;