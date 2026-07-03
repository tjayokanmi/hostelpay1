-- Down Migration: Completely removes the ledger tracking system safely
DROP INDEX IF EXISTS idx_payments_student;
DROP TABLE IF EXISTS payments;

-- Note: pgcrypto is deliberately left intact for future scale