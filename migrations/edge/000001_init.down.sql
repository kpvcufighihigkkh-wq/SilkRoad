-- IGH Silkroad - Edge Database Rollback
-- Drop all tables and triggers

DROP TRIGGER IF EXISTS update_sync_logs_updated_at;
DROP TRIGGER IF EXISTS update_product_configs_updated_at;
DROP TRIGGER IF EXISTS update_bobbin_grades_updated_at;
DROP TRIGGER IF EXISTS update_bobbins_updated_at;
DROP TRIGGER IF EXISTS update_lots_updated_at;

DROP TABLE IF EXISTS sync_logs;
DROP TABLE IF EXISTS product_configs;
DROP TABLE IF EXISTS bobbin_grades;
DROP TABLE IF EXISTS bobbins;
DROP TABLE IF EXISTS lots;
