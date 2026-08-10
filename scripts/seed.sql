-- =============================================================================
-- 1. INCIDENT TYPES
-- =============================================================================
INSERT INTO public.incident_types (id, name, color, icon, is_active)
VALUES 
    ('22222222-2222-2222-2222-222222222222', 'Physical Threat', '#FF0000', 'shield-alert', true),
    ('22222222-2222-2222-2222-333333333333', 'Medical Emergency', '#E63946', 'heart-pulse', true),
    ('22222222-2222-2222-2222-444444444444', 'Theft / Robbery', '#FFB703', 'bag-personal', true),
    ('22222222-2222-2222-2222-555555555555', 'Lost / Disoriented', '#457B9D', 'map-marker-question', true)
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 2. PATTAYA SECTORS (PostGIS Polygons)
-- =============================================================================
INSERT INTO public.sectors (id, name, description, area)
VALUES 
    (
        '33333333-3333-3333-3333-111111111111', 
        'Pattaya Central & Walking St', 
        'Central Pattaya area, including Beach Rd, Second Rd, and Walking Street',
        ST_PolygonFromText('POLYGON((100.865 12.920, 100.890 12.920, 100.890 12.945, 100.865 12.945, 100.865 12.920))', 4326)
    ),
    (
        '33333333-3333-3333-3333-222222222222', 
        'Jomtien Beach', 
        'Jomtien area, stretching from Pratumnak Hill southward along the coastline',
        ST_PolygonFromText('POLYGON((100.860 12.870, 100.895 12.870, 100.895 12.915, 100.860 12.915, 100.860 12.870))', 4326)
    ),
    (
        '33333333-3333-3333-3333-333333333333', 
        'Naklua & Wongamat', 
        'North Pattaya zone, covering Naklua region and Wongamat residential area',
        ST_PolygonFromText('POLYGON((100.875 12.946, 100.910 12.946, 100.910 12.975, 100.875 12.975, 100.875 12.946))', 4326)
    )
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 3. UNIFIED USERS (Tourists, Dispatchers, RRT)
-- Default bcrypt hash payload mockup stands for: "password123"
-- =============================================================================
INSERT INTO public.users (id, phone, password_hash, role, fullname)
VALUES 
    -- Tourists / Reporters
    ('11111111-1111-1111-1111-111111111111', '+66012345678', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'tourist', 'Liam Henderson'),
    ('11111111-1111-1111-1111-222222222222', '+66098765432', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'tourist', 'Elena Rodriguez'),
    
    -- Dispatchers
    ('44444444-1111-1111-1111-111111111111', '+66088888881', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'dispatcher', 'Somchai Jaidee'),
    ('44444444-1111-1111-1111-222222222222', '+66088888882', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'dispatcher', 'Kanya Raksa'),

    -- Mobile Response Teams (RRT)
    ('55555555-1111-1111-1111-111111111111', '+66077777771', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'rrt', 'RRT Crew Alpha (Bike-01)'),
    ('55555555-1111-1111-1111-222222222222', '+66077777772', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'rrt', 'RRT Crew Bravo (Truck-01)'),
    ('55555555-1111-1111-1111-333333333333', '+66077777773', '$2a$10$qWjpisQC0NlLi3WcT1j.2ukjXIzGJcDAWt6m2Hyzfg6i0vIbqWyGC', 'rrt', 'RRT Crew Charlie (Bike-02)')
ON CONFLICT (id) DO UPDATE SET password_hash = EXCLUDED.password_hash;

-- =============================================================================
-- 4. DISPATCHERS PROFILE
-- =============================================================================
INSERT INTO public.dispatchers (id, station_name, sector_id, is_active)
VALUES 
    ('44444444-1111-1111-1111-111111111111', 'Pattaya City Hall Command Center', '33333333-3333-3333-3333-111111111111', true),
    ('44444444-1111-1111-1111-222222222222', 'Jomtien Substation', '33333333-3333-3333-3333-222222222222', true)
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 5. RAPID RESPONSE TEAMS PROFILE (RRT)
-- =============================================================================
INSERT INTO public.rrt (id, status, sector_id)
VALUES 
    ('55555555-1111-1111-1111-111111111111', 'ready', '33333333-3333-3333-3333-111111111111'),
    ('55555555-1111-1111-1111-222222222222', 'en_route', '33333333-3333-3333-3333-222222222222'),
    ('55555555-1111-1111-1111-333333333333', 'offline', '33333333-3333-3333-3333-333333333333')
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 6. INCIDENTS
-- =============================================================================
INSERT INTO public.incidents (
    id, tourist_id, rrt_id, dispatcher_id, type_id, sector_id, status, battery, description, coords, created_at
) VALUES 
    (
        -- Open incident on Walking Street (Liam)
        '99999999-9999-9999-9999-111111111111',
        '11111111-1111-1111-1111-111111111111',
        NULL, -- No crew dispatched yet
        '44444444-1111-1111-1111-111111111111', -- Somchai
        '22222222-2222-2222-2222-222222222222', -- Physical Threat
        '33333333-3333-3333-3333-111111111111', -- Central Sector
        'created',
        18,
        'Aggressive local group harassing tourists near the main entrance of Walking Street.',
        ST_SetSRID(ST_MakePoint(100.8729, 12.9255), 4326),
        NOW() - INTERVAL '4 minutes'
    ),
    (
        -- In-progress incident at Jomtien Beach (Elena)
        '99999999-9999-9999-9999-222222222222',
        '11111111-1111-1111-1111-222222222222',
        '55555555-1111-1111-1111-222222222222', -- RRT Bravo assigned
        '44444444-1111-1111-1111-222222222222', -- Kanya
        '22222222-2222-2222-2222-333333333333', -- Medical Emergency
        '33333333-3333-3333-3333-222222222222', -- Jomtien Sector
        'en_route',
        74,
        'Severe heat stroke symptoms reported near the Jomtien Night Market.',
        ST_SetSRID(ST_MakePoint(100.8762, 12.8945), 4326),
        NOW() - INTERVAL '15 minutes'
    )
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 7. INCIDENT LOGS (Audit Trail)
-- =============================================================================
INSERT INTO public.incident_logs (id, incident_id, status, changed_by, comment, created_at)
VALUES 
    (
        '88888888-8888-8888-8888-111111111111',
        '99999999-9999-9999-9999-111111111111',
        'created',
        '11111111-1111-1111-1111-111111111111',
        'SOS alert broadcasted via mobile client application.',
        NOW() - INTERVAL '4 minutes'
    ),
    (
        '88888888-8888-8888-8888-222222222222',
        '99999999-9999-9999-9999-222222222222',
        'created',
        '11111111-1111-1111-1111-222222222222',
        'SOS broadcasted. Requiring immediate medical attention.',
        NOW() - INTERVAL '15 minutes'
    ),
    (
        '88888888-8888-8888-8888-333333333333',
        '99999999-9999-9999-9999-222222222222',
        'en_route',
        '44444444-1111-1111-1111-222222222222',
        'RRT Crew Bravo deployed with emergency truck setup.',
        NOW() - INTERVAL '12 minutes'
    )
ON CONFLICT (id) DO NOTHING;
