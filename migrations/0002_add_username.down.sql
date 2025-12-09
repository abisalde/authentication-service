-- Remove username column from users table
DROP INDEX IF EXISTS idx_users_username_lookup ON users;
DROP INDEX IF EXISTS idx_users_username ON users;
ALTER TABLE users DROP COLUMN username;
