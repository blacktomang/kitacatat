-- +goose Up
-- +goose StatementBegin
CREATE TABLE transactions (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     text        NOT NULL,
    amount      numeric     NOT NULL,
    type        text        NOT NULL CHECK (type IN ('income', 'expense')),
    category    text        NOT NULL,
    description text,
    occurred_at timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX transactions_occurred_at_idx ON transactions (occurred_at DESC);
CREATE INDEX transactions_user_id_idx ON transactions (user_id);

-- Row Level Security: the dashboard talks to Postgres directly via the
-- Supabase anon key, so we must enable RLS and add an explicit policy.
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;

-- PERMISSIVE start for a 2-person family deployment: the anon key can read
-- all rows. This is acceptable because the project is private and the anon
-- key is only shared between the two family members.
--
-- TO TIGHTEN LATER:
--   1. Turn on Supabase Auth (email/magic-link) for both family members.
--   2. Add a `user_id` mapping to auth.uid() (e.g. store the Supabase auth
--      uuid instead of the Telegram id, or keep a lookup table).
--   3. Replace this policy with:
--        USING (auth.uid()::text = user_id)
--      and drop the anon grant so only authenticated users can read.
--   4. Writes still come exclusively from the Go bot using the service-role
--      / direct Postgres connection, which bypasses RLS — so no write policy
--      is needed here.
CREATE POLICY "family read access"
    ON transactions
    FOR SELECT
    USING (true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transactions;
-- +goose StatementEnd
