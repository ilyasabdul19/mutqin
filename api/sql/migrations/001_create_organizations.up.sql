CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    city TEXT,
    country TEXT NOT NULL DEFAULT 'SO',
    tier TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'asaasi', 'pro')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    logo_url TEXT,
    description TEXT,
    schedule JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
