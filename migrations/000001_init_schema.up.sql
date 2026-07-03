-- Up Migration: Configures extensions and creates the bulletproof ledger tracking table
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_reference TEXT UNIQUE NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id),

    occupancy_type TEXT NOT NULL
        CONSTRAINT chk_occupancy_type CHECK (occupancy_type IN ('single', 'shared')),

    amount_paid NUMERIC(12, 2) NOT NULL
        CONSTRAINT chk_amount_paid CHECK (amount_paid > 0),

    payment_status TEXT NOT NULL DEFAULT 'PENDING'
        CONSTRAINT chk_payment_status CHECK (payment_status IN ('PENDING', 'SUCCESS', 'FAILED')),

    provider_reference TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payments_tenant ON payments(tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_ref
    ON payments(provider_reference) WHERE provider_reference IS NOT NULL;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();