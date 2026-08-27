-- Reverts 001_create_sessions_table.up.sql
DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP FUNCTION IF EXISTS set_sessions_updated_at();
DROP TABLE IF EXISTS sessions;
