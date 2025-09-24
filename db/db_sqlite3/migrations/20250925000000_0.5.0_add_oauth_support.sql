-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE users ADD COLUMN oauth_provider VARCHAR(50);
ALTER TABLE users ADD COLUMN oauth_id VARCHAR(255);
ALTER TABLE users ADD COLUMN email VARCHAR(255);
ALTER TABLE users ADD COLUMN display_name VARCHAR(255);

-- Create unique index for OAuth users (composite key)
CREATE UNIQUE INDEX idx_users_oauth ON users(oauth_provider, oauth_id)
WHERE oauth_provider IS NOT NULL AND oauth_id IS NOT NULL;

-- Create OAuth tokens table for secure token storage
CREATE TABLE oauth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    access_token_hash VARCHAR(255), -- encrypted token
    refresh_token_hash VARCHAR(255), -- encrypted refresh token
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_oauth_tokens_user_provider ON oauth_tokens(user_id, provider);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_oauth_tokens_user_provider;
DROP TABLE IF EXISTS oauth_tokens;
DROP INDEX IF EXISTS idx_users_oauth;
ALTER TABLE users DROP COLUMN display_name;
ALTER TABLE users DROP COLUMN email;
ALTER TABLE users DROP COLUMN oauth_id;
ALTER TABLE users DROP COLUMN oauth_provider;