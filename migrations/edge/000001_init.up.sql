-- IGH Silkroad - Edge Database Initialization
-- SQLite

-- Enable foreign keys
PRAGMA foreign_keys = ON;

-- Lots table
CREATE TABLE IF NOT EXISTS lots (
    id TEXT PRIMARY KEY,
    lot_number TEXT NOT NULL UNIQUE,
    order_id TEXT NOT NULL,
    product_type TEXT NOT NULL CHECK (product_type IN ('FDY', 'POY', 'DTY')),
    product_spec TEXT,
    planned_quantity INTEGER NOT NULL CHECK (planned_quantity > 0),
    actual_quantity INTEGER NOT NULL DEFAULT 0 CHECK (actual_quantity >= 0),
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'paused', 'completed', 'cancelled')),
    start_time DATETIME,
    end_time DATETIME,
    synced INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_lots_status ON lots(status);
CREATE INDEX idx_lots_synced ON lots(synced);
CREATE INDEX idx_lots_created_at ON lots(created_at);

-- Bobbins table
CREATE TABLE IF NOT EXISTS bobbins (
    id TEXT PRIMARY KEY,
    bobbin_number TEXT NOT NULL UNIQUE,
    lot_id TEXT NOT NULL,
    spinning_position INTEGER NOT NULL CHECK (spinning_position > 0),
    gross_weight REAL NOT NULL CHECK (gross_weight > 0),
    net_weight REAL NOT NULL CHECK (net_weight > 0),
    tare_weight REAL CHECK (tare_weight >= 0),
    grade TEXT,
    status TEXT NOT NULL DEFAULT 'producing' CHECK (status IN ('producing', 'completed', 'packed', 'shipped')),
    label_printed INTEGER NOT NULL DEFAULT 0,
    printed_at DATETIME,
    completed_at DATETIME,
    synced INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (lot_id) REFERENCES lots(id) ON DELETE CASCADE
);

CREATE INDEX idx_bobbins_lot_id ON bobbins(lot_id);
CREATE INDEX idx_bobbins_spinning_position ON bobbins(spinning_position);
CREATE INDEX idx_bobbins_status ON bobbins(status);
CREATE INDEX idx_bobbins_synced ON bobbins(synced);
CREATE INDEX idx_bobbins_lot_status ON bobbins(lot_id, status);
CREATE INDEX idx_bobbins_lot_position ON bobbins(lot_id, spinning_position);

-- Bobbin grades table
CREATE TABLE IF NOT EXISTS bobbin_grades (
    id TEXT PRIMARY KEY,
    bobbin_id TEXT NOT NULL UNIQUE,
    inspector_id TEXT,
    grade TEXT NOT NULL,
    inspection_details TEXT,
    scores TEXT,
    final_score REAL CHECK (final_score >= 0),
    is_qualified INTEGER NOT NULL DEFAULT 1,
    defect_reason TEXT,
    inspected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    synced INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bobbin_grades_bobbin_id ON bobbin_grades(bobbin_id);
CREATE INDEX idx_bobbin_grades_grade ON bobbin_grades(grade);
CREATE INDEX idx_bobbin_grades_is_qualified ON bobbin_grades(is_qualified);
CREATE INDEX idx_bobbin_grades_synced ON bobbin_grades(synced);

-- Product configs table (read-only cache from center)
CREATE TABLE IF NOT EXISTS product_configs (
    id TEXT PRIMARY KEY,
    product_type TEXT NOT NULL CHECK (product_type IN ('FDY', 'POY', 'DTY')),
    product_spec TEXT NOT NULL,
    process_params TEXT,
    quality_standards TEXT,
    grade_system TEXT,
    target_weight REAL CHECK (target_weight > 0),
    weight_tolerance REAL CHECK (weight_tolerance > 0),
    is_active INTEGER NOT NULL DEFAULT 1,
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (product_type, product_spec)
);

CREATE INDEX idx_product_configs_product_type ON product_configs(product_type);
CREATE INDEX idx_product_configs_product_spec ON product_configs(product_spec);
CREATE INDEX idx_product_configs_is_active ON product_configs(is_active);

-- Sync logs table
CREATE TABLE IF NOT EXISTS sync_logs (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('lot', 'bobbin', 'bobbin_grade')),
    entity_id TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed', 'conflict')),
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    error_message TEXT,
    data_snapshot TEXT,
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sync_logs_entity_type ON sync_logs(entity_type);
CREATE INDEX idx_sync_logs_entity_id ON sync_logs(entity_id);
CREATE INDEX idx_sync_logs_status ON sync_logs(status);
CREATE INDEX idx_sync_logs_created_at ON sync_logs(created_at);
CREATE INDEX idx_sync_logs_entity_type_id ON sync_logs(entity_type, entity_id);
CREATE INDEX idx_sync_logs_status_created ON sync_logs(status, created_at);

-- Update timestamp triggers
CREATE TRIGGER update_lots_updated_at
AFTER UPDATE ON lots
BEGIN
    UPDATE lots SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_bobbins_updated_at
AFTER UPDATE ON bobbins
BEGIN
    UPDATE bobbins SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_bobbin_grades_updated_at
AFTER UPDATE ON bobbin_grades
BEGIN
    UPDATE bobbin_grades SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_product_configs_updated_at
AFTER UPDATE ON product_configs
BEGIN
    UPDATE product_configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_sync_logs_updated_at
AFTER UPDATE ON sync_logs
BEGIN
    UPDATE sync_logs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
