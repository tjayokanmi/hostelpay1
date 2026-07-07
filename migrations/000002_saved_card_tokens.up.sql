CREATE TABLE IF NOT EXISTS saved_card_tokens (
    student_identifier TEXT PRIMARY KEY,
    token_key TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
