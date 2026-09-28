-- IGH Silkroad - Center Database Initialization
-- PostgreSQL 16

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    customer_name VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_customer ON projects(customer_name);
CREATE INDEX idx_projects_created_at ON projects(created_at DESC);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number VARCHAR(50) NOT NULL UNIQUE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    product_type VARCHAR(20) NOT NULL CHECK (product_type IN ('FDY', 'POY', 'DTY')),
    product_spec VARCHAR(100),
    target_quantity INTEGER NOT NULL CHECK (target_quantity > 0),
    actual_quantity INTEGER NOT NULL DEFAULT 0 CHECK (actual_quantity >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'cancelled')),
    priority INTEGER NOT NULL DEFAULT 0,
    delivery_date DATE,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_project_id ON orders(project_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_product_type ON orders(product_type);
CREATE INDEX idx_orders_priority ON orders(priority DESC);
CREATE INDEX idx_orders_delivery_date ON orders(delivery_date);

-- Lots table
CREATE TABLE IF NOT EXISTS lots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lot_number VARCHAR(50) NOT NULL UNIQUE,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    product_type VARCHAR(20) NOT NULL CHECK (product_type IN ('FDY', 'POY', 'DTY')),
    product_spec VARCHAR(100),
    planned_quantity INTEGER NOT NULL CHECK (planned_quantity > 0),
    actual_quantity INTEGER NOT NULL DEFAULT 0 CHECK (actual_quantity >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'paused', 'completed', 'cancelled')),
    spinning_line_id UUID,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_lots_order_id ON lots(order_id);
CREATE INDEX idx_lots_status ON lots(status);
CREATE INDEX idx_lots_product_type ON lots(product_type);
CREATE INDEX idx_lots_spinning_line_id ON lots(spinning_line_id);
CREATE INDEX idx_lots_created_at ON lots(created_at DESC);

-- Bobbins table
CREATE TABLE IF NOT EXISTS bobbins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bobbin_number VARCHAR(50) NOT NULL UNIQUE,
    lot_id UUID REFERENCES lots(id) ON DELETE CASCADE,
    spinning_position INTEGER NOT NULL CHECK (spinning_position > 0),
    gross_weight DOUBLE PRECISION NOT NULL CHECK (gross_weight > 0),
    net_weight DOUBLE PRECISION NOT NULL CHECK (net_weight > 0),
    tare_weight DOUBLE PRECISION CHECK (tare_weight >= 0),
    grade VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'producing' CHECK (status IN ('producing', 'completed', 'graded', 'packed', 'shipped')),
    pallet_id UUID,
    carton_id UUID,
    label_printed BOOLEAN NOT NULL DEFAULT FALSE,
    printed_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bobbins_lot_id ON bobbins(lot_id);
CREATE INDEX idx_bobbins_spinning_position ON bobbins(spinning_position);
CREATE INDEX idx_bobbins_status ON bobbins(status);
CREATE INDEX idx_bobbins_grade ON bobbins(grade);
CREATE INDEX idx_bobbins_pallet_id ON bobbins(pallet_id);
CREATE INDEX idx_bobbins_carton_id ON bobbins(carton_id);
CREATE INDEX idx_bobbins_lot_status ON bobbins(lot_id, status);
CREATE INDEX idx_bobbins_lot_position ON bobbins(lot_id, spinning_position);

-- Bobbin grades table
CREATE TABLE IF NOT EXISTS bobbin_grades (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bobbin_id UUID NOT NULL UNIQUE,
    inspector_id UUID,
    grade VARCHAR(20) NOT NULL,
    inspection_details JSONB,
    scores JSONB,
    final_score DOUBLE PRECISION CHECK (final_score >= 0),
    is_qualified BOOLEAN NOT NULL DEFAULT TRUE,
    defect_reason TEXT,
    notes TEXT,
    inspected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bobbin_grades_bobbin_id ON bobbin_grades(bobbin_id);
CREATE INDEX idx_bobbin_grades_grade ON bobbin_grades(grade);
CREATE INDEX idx_bobbin_grades_is_qualified ON bobbin_grades(is_qualified);
CREATE INDEX idx_bobbin_grades_inspector_id ON bobbin_grades(inspector_id);
CREATE INDEX idx_bobbin_grades_inspected_at ON bobbin_grades(inspected_at);

-- Pallets table
CREATE TABLE IF NOT EXISTS pallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pallet_number VARCHAR(50) NOT NULL UNIQUE,
    lot_id UUID,
    bobbin_count INTEGER NOT NULL DEFAULT 0 CHECK (bobbin_count >= 0),
    total_weight DOUBLE PRECISION CHECK (total_weight >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'packing' CHECK (status IN ('packing', 'packed', 'shipped')),
    label_printed BOOLEAN NOT NULL DEFAULT FALSE,
    printed_at TIMESTAMP,
    packed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_pallets_lot_id ON pallets(lot_id);
CREATE INDEX idx_pallets_status ON pallets(status);
CREATE INDEX idx_pallets_packed_at ON pallets(packed_at);
CREATE INDEX idx_pallets_created_at ON pallets(created_at);

-- Cartons table
CREATE TABLE IF NOT EXISTS cartons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    carton_number VARCHAR(50) NOT NULL UNIQUE,
    lot_id UUID,
    bobbin_count INTEGER NOT NULL DEFAULT 0 CHECK (bobbin_count >= 0),
    total_weight DOUBLE PRECISION CHECK (total_weight >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'packing' CHECK (status IN ('packing', 'packed', 'shipped')),
    label_printed BOOLEAN NOT NULL DEFAULT FALSE,
    printed_at TIMESTAMP,
    packed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cartons_lot_id ON cartons(lot_id);
CREATE INDEX idx_cartons_status ON cartons(status);
CREATE INDEX idx_cartons_packed_at ON cartons(packed_at);
CREATE INDEX idx_cartons_created_at ON cartons(created_at);

-- Spinning lines table
CREATE TABLE IF NOT EXISTS spinning_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    line_name VARCHAR(100) NOT NULL UNIQUE,
    line_number VARCHAR(50),
    location VARCHAR(100),
    capacity INTEGER CHECK (capacity > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'maintenance', 'offline')),
    current_lot_id UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_spinning_lines_status ON spinning_lines(status);
CREATE INDEX idx_spinning_lines_current_lot_id ON spinning_lines(current_lot_id);

-- Product configs table
CREATE TABLE IF NOT EXISTS product_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_type VARCHAR(20) NOT NULL CHECK (product_type IN ('FDY', 'POY', 'DTY')),
    product_spec VARCHAR(100) NOT NULL,
    process_params JSONB,
    quality_standards JSONB,
    grade_system JSONB,
    target_weight DOUBLE PRECISION CHECK (target_weight > 0),
    weight_tolerance DOUBLE PRECISION CHECK (weight_tolerance > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (product_type, product_spec)
);

CREATE INDEX idx_product_configs_product_type ON product_configs(product_type);
CREATE INDEX idx_product_configs_product_spec ON product_configs(product_spec);
CREATE INDEX idx_product_configs_is_active ON product_configs(is_active);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'operator', 'inspector', 'viewer')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_is_active ON users(is_active);

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply update triggers to all tables
CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_lots_updated_at BEFORE UPDATE ON lots FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_bobbins_updated_at BEFORE UPDATE ON bobbins FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_bobbin_grades_updated_at BEFORE UPDATE ON bobbin_grades FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_pallets_updated_at BEFORE UPDATE ON pallets FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_cartons_updated_at BEFORE UPDATE ON cartons FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_spinning_lines_updated_at BEFORE UPDATE ON spinning_lines FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_product_configs_updated_at BEFORE UPDATE ON product_configs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
