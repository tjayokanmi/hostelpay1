-- Up Migration: Creates the core payments ledger tracking table
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_reference VARCHAR(100) UNIQUE NOT NULL,
    student_identifier VARCHAR(150) NOT NULL,
    block VARCHAR(50) NOT NULL,          -- 'white house', 'green house', 'blue house'
    floor_level VARCHAR(50) NOT NULL,    -- 'Ground Floor', 'Middle floor', 'Top floor'
    room_number VARCHAR(20) NOT NULL,
    occupancy_type VARCHAR(10) NOT NULL, -- 'single' or 'shared'
    amount_paid NUMERIC(12, 2) NOT NULL,  -- Numeric type for perfectly precise fiat money balances
    payment_status VARCHAR(20) NOT NULL, -- 'PENDING', 'SUCCESS', 'FAILED'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexing order references allows instantaneous lookups when Nomba fires webhooks later
CREATE INDEX IF NOT EXISTS idx_payments_order_ref ON payments(order_reference);