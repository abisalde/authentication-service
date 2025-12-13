-- Add username column to users table
ALTER TABLE users ADD COLUMN username VARCHAR(30) NULL AFTER email;

-- Add unique index on username
CREATE UNIQUE INDEX idx_users_username ON users(username);

-- Add index for faster lookups
CREATE INDEX idx_users_username_lookup ON users(username) WHERE username IS NOT NULL;
