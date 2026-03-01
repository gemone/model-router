-- Migration: 001_destroy_rebuild.down.sql
-- Description: Rollback script for destructive rebuild
-- WARNING: This is a NO-OP rollback since data was already destroyed
-- The forward migration deletes all data, which cannot be recovered
-- Usage: sqlite3 data.db < migrations/001_destroy_rebuild.down.sql

-- ============================================
-- ROLLBACK NOTICE
-- ============================================

-- This migration is DESTRUCTIVE and CANNOT be rolled back safely.
-- The forward migration deleted all data, which cannot be recovered.

-- Options after running this migration:
-- 1. Restore from backup (if you have one)
-- 2. Manually recreate data
-- 3. Accept the data loss and continue with fresh schema

-- ============================================
-- SCHEMA INFORMATION
-- ============================================

-- If you need to verify the current schema, use these queries:

-- List all tables
SELECT name AS 'Tables in database'
FROM sqlite_master
WHERE type='table'
ORDER BY name;

-- Show schema for a specific table (replace 'profiles' with table name)
-- .schema profiles

-- Show all indexes
SELECT name AS 'Indexes in database'
FROM sqlite_master
WHERE type='index'
AND name NOT LIKE 'sqlite_%'
ORDER BY name;

-- ============================================
-- BACKUP DATA BEFORE MIGRATION
-- ============================================

-- To prevent data loss in future migrations, always:
-- 1. Create a backup: cp data.db data.db.backup
-- 2. Test migration on backup first
-- 3. Use non-destructive migrations when possible
-- 4. Keep migration history in version control

-- Example backup command (run from shell, not SQL):
-- cp ~/.model-router/data.db ~/.model-router/data.db.backup.$(date +%Y%m%d_%H%M%S)

-- ============================================
-- CURRENT STATE AFTER MIGRATION
-- ============================================

-- After running the up migration, you have these tables:
-- - settings: System configuration
-- - profiles: Routing profiles
-- - providers: Model providers
-- - models: Model configurations
-- - compression_model_groups: Compression model groups
-- - composite_auto_models: Composite auto models
-- - route_rules: Routing rules
-- - api_keys: API keys
-- - request_logs: Request logs
-- - stats: Statistics
-- - test_results: Test results
-- - sessions: Session contexts
-- - session_messages: Session messages

-- All tables are empty after this migration.
-- You need to seed initial data through the API or admin interface.

SELECT 'Rollback notice: This migration cannot be undone.' AS message;
SELECT 'Data was permanently deleted. Restore from backup if needed.' AS details;
SELECT 'Current schema is active and ready for new data.' AS status;
