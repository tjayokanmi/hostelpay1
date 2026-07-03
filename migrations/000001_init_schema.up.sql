-- Up Migration: Configures extensions and creates the flat ledger tracking table
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_reference TEXT UNIQUE NOT NULL,

    -- The human identity of the tenant
    student_identifier TEXT NOT NULL,
    
    -- Specific location constraints (Lowercased for consistent querying)
    block TEXT NOT NULL 
        CONSTRAINT chk_block CHECK (block IN ('white house', 'green house', 'blue house')),
    floor_level TEXT NOT NULL 
        CONSTRAINT chk_floor_level CHECK (floor_level IN ('ground floor', 'middle floor', 'top floor')),
    room_number TEXT NOT NULL,
    
    -- Pricing strategy validation
    occupancy_type TEXT NOT NULL
        CONSTRAINT chk_occupancy_type CHECK (occupancy_type IN ('single', 'shared')),
        
    -- Strict financial guardrails
    amount_paid NUMERIC(12, 2) NOT NULL
        CONSTRAINT chk_amount_paid CHECK (amount_paid > 0),
        
    -- Valid payment states accepted by the application
    payment_status TEXT NOT NULL DEFAULT 'PENDING'
        CONSTRAINT chk_payment_status CHECK (payment_status IN ('PENDING', 'SUCCESS', 'FAILED')),
        
    -- Captures Nomba's official internal reference for audits
    provider_reference TEXT,
    
    -- Timestamps utilizing explicit timezones (TIMESTAMPTZ) and NOT NULL
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexing for instantaneous student ledger lookups
CREATE INDEX IF NOT EXISTS idx_payments_student ON payments(student_identifier);