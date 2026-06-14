-- +goose Up
-- +goose StatementBegin

-- ── profiles ────────────────────────────────────────────────────────────────
-- One row per dashboard (Supabase Auth) user. telegram_id is filled in once the
-- user links their Telegram account, and is what the bot checks instead of a
-- hardcoded ALLOWED_USER_IDS list.
CREATE TABLE profiles (
    id           uuid        PRIMARY KEY REFERENCES auth.users (id) ON DELETE CASCADE,
    telegram_id  bigint      UNIQUE,
    display_name text,
    created_at   timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE profiles ENABLE ROW LEVEL SECURITY;

-- A logged-in user can only see and edit their own profile row.
CREATE POLICY "own profile read"   ON profiles FOR SELECT USING (auth.uid() = id);
CREATE POLICY "own profile update" ON profiles FOR UPDATE USING (auth.uid() = id);

-- +goose StatementEnd

-- Auto-create a profile whenever a new auth user signs up.
-- +goose StatementBegin
CREATE FUNCTION handle_new_user()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    INSERT INTO public.profiles (id, display_name)
    VALUES (NEW.id, COALESCE(NEW.raw_user_meta_data ->> 'name', NEW.email));
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER on_auth_user_created
    AFTER INSERT ON auth.users
    FOR EACH ROW EXECUTE FUNCTION handle_new_user();
-- +goose StatementEnd

-- ── telegram_link_codes ─────────────────────────────────────────────────────
-- Short-lived, single-use codes the dashboard hands out to link a Telegram
-- account to a profile. Created via the request_telegram_link_code() RPC and
-- consumed by the bot when the user sends /start <code>.
-- +goose StatementBegin
CREATE TABLE telegram_link_codes (
    code        text        PRIMARY KEY,
    profile_id  uuid        NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz
);

ALTER TABLE telegram_link_codes ENABLE ROW LEVEL SECURITY;
-- No client policies: rows are created through the SECURITY DEFINER RPC below
-- and read/updated only by the bot via the direct Postgres connection (which
-- bypasses RLS). With RLS enabled and no policy, the anon/auth roles cannot
-- read these codes.
-- +goose StatementEnd

-- RPC: the authenticated dashboard user requests a fresh link code for itself.
-- +goose StatementBegin
CREATE FUNCTION request_telegram_link_code()
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_code text;
BEGIN
    IF auth.uid() IS NULL THEN
        RAISE EXCEPTION 'not authenticated';
    END IF;

    v_code := upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 8));

    INSERT INTO telegram_link_codes (code, profile_id, expires_at)
    VALUES (v_code, auth.uid(), now() + interval '15 minutes');

    RETURN v_code;
END;
$$;

GRANT EXECUTE ON FUNCTION request_telegram_link_code() TO authenticated;
-- +goose StatementEnd

-- ── transactions now belong to a profile ────────────────────────────────────
-- +goose StatementBegin
ALTER TABLE transactions
    ALTER COLUMN user_id TYPE uuid USING user_id::uuid;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES profiles (id) ON DELETE CASCADE;

-- Replace the permissive read policy with: any logged-in family member may read
-- all transactions (shared household view). The bot writes via the direct
-- Postgres connection, which bypasses RLS, so no write policy is needed.
--
-- TO ISOLATE PER USER instead, change the USING clause to:
--     USING (auth.uid() = user_id)
DROP POLICY IF EXISTS "family read access" ON transactions;
CREATE POLICY "authenticated family read"
    ON transactions FOR SELECT
    USING (auth.uid() IS NOT NULL);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_user_id_fkey;
DROP POLICY IF EXISTS "authenticated family read" ON transactions;
ALTER TABLE transactions ALTER COLUMN user_id TYPE text USING user_id::text;
CREATE POLICY "family read access" ON transactions FOR SELECT USING (true);

DROP FUNCTION IF EXISTS request_telegram_link_code();
DROP TABLE IF EXISTS telegram_link_codes;
DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
DROP FUNCTION IF EXISTS handle_new_user();
DROP TABLE IF EXISTS profiles;
-- +goose StatementEnd
