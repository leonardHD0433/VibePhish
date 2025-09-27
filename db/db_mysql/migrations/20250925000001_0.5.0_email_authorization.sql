-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Create authorized_emails table for email-based access control
CREATE TABLE authorized_emails (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    normalized_email VARCHAR(255) NOT NULL UNIQUE, -- lowercase, trimmed for lookups
    status ENUM('active', 'suspended', 'revoked') NOT NULL DEFAULT 'active',
    role_id BIGINT,
    default_role VARCHAR(50) DEFAULT 'user', -- fallback role name
    created_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NULL, -- optional expiration
    last_used_at TIMESTAMP NULL, -- track usage
    notes TEXT, -- admin notes

    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE SET NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for performance
CREATE INDEX idx_authorized_emails_normalized ON authorized_emails(normalized_email, status);
CREATE INDEX idx_authorized_emails_status ON authorized_emails(status);
CREATE INDEX idx_authorized_emails_expires ON authorized_emails(expires_at);

-- Create email authorization audit log
CREATE TABLE email_authorization_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    normalized_email VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL, -- check, grant, deny, add, remove, suspend
    result ENUM('success', 'denied', 'error') NOT NULL,
    ip_address VARCHAR(45), -- support IPv6
    user_agent TEXT,
    user_id BIGINT, -- if authenticated user
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    details TEXT, -- additional context

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for audit log queries
CREATE INDEX idx_email_auth_logs_email ON email_authorization_logs(normalized_email, created_at);
CREATE INDEX idx_email_auth_logs_created ON email_authorization_logs(created_at);
CREATE INDEX idx_email_auth_logs_action ON email_authorization_logs(action, result);

-- Create email domains table for domain-based authorization (optional)
CREATE TABLE authorized_domains (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    domain VARCHAR(255) NOT NULL UNIQUE,
    status ENUM('active', 'suspended', 'revoked') NOT NULL DEFAULT 'active',
    default_role VARCHAR(50) DEFAULT 'user',
    created_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,

    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
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