-- Down Migration: Completely removes the ledger tracking system
DROP INDEX IF EXISTS idx_payments_order_ref;
DROP TABLE IF EXISTS payments;