CREATE TABLE IF NOT EXISTS entity_images (
    id SERIAL PRIMARY KEY,
    entity_name TEXT UNIQUE NOT NULL,
    image_url TEXT,
    summary TEXT,
    source TEXT DEFAULT 'wikipedia',
    last_checked TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
