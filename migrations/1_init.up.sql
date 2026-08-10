-- Enable the PostGIS extension (requires database superuser privileges).
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE incident_types (
    id UUID PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    color VARCHAR(10) NOT NULL,
    icon VARCHAR(50),
    is_active BOOLEAN DEFAULT true
);

CREATE TABLE sectors (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    -- If there are no sectors, it may be NULL.
    area GEOMETRY(Polygon, 4326), 
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for instant "which polygon contains these coordinates" lookups.
CREATE INDEX idx_sectors_area ON sectors USING GIST (area);

-- Unified user table
CREATE TABLE users (
    id UUID PRIMARY KEY,
    phone VARCHAR(20) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    role VARCHAR(20) NOT NULL,
    fullname VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE dispatchers (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    station_name VARCHAR(100),
    sector_id UUID REFERENCES sectors(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE rrt (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'offline',
    sector_id UUID REFERENCES sectors(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE SEQUENCE IF NOT EXISTS incidents_number_seq;

CREATE TABLE incidents (
    id UUID PRIMARY KEY,
    number INT NOT NULL DEFAULT nextval('incidents_number_seq'),
    tourist_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rrt_id UUID REFERENCES rrt(id) ON DELETE SET NULL,
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE SET NULL,
    type_id UUID REFERENCES incident_types(id) ON DELETE SET NULL,
    sector_id UUID REFERENCES sectors(id) ON DELETE SET NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'created',
    battery INT,
    description TEXT,
    coords GEOMETRY(Point, 4326) NOT NULL, 
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

-- Spatial index for searching incidents by geo-grid
CREATE INDEX idx_incidents_coords ON incidents USING GIST (coords);
CREATE INDEX idx_rrt_status ON rrt(status);
CREATE UNIQUE INDEX idx_incidents_number ON incidents(number);

CREATE TABLE incident_logs (
    id UUID PRIMARY KEY,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL,
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    comment TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
