-- IGH Silkroad - Center Database Rollback
-- Drop all tables and extensions

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_product_configs_updated_at ON product_configs;
DROP TRIGGER IF EXISTS update_spinning_lines_updated_at ON spinning_lines;
DROP TRIGGER IF EXISTS update_cartons_updated_at ON cartons;
DROP TRIGGER IF EXISTS update_pallets_updated_at ON pallets;
DROP TRIGGER IF EXISTS update_bobbin_grades_updated_at ON bobbin_grades;
DROP TRIGGER IF EXISTS update_bobbins_updated_at ON bobbins;
DROP TRIGGER IF EXISTS update_lots_updated_at ON lots;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS product_configs CASCADE;
DROP TABLE IF EXISTS spinning_lines CASCADE;
DROP TABLE IF EXISTS cartons CASCADE;
DROP TABLE IF EXISTS pallets CASCADE;
DROP TABLE IF EXISTS bobbin_grades CASCADE;
DROP TABLE IF EXISTS bobbins CASCADE;
DROP TABLE IF EXISTS lots CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS projects CASCADE;

DROP EXTENSION IF EXISTS "uuid-ossp";
