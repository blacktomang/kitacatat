-- kitacatat baseline schema
--
-- Access model: dashboard users authenticate via Supabase Auth. A Telegram
-- account is linked to a profile, and that linkage (not a hardcoded id list)
-- is what authorizes the bot. The bot writes via the direct Postgres
-- connection (service/owner role) which bypasses RLS; the dashboard reads via
-- the authenticated role, gated by the policies below.

-- ── profiles ────────────────────────────────────────────────────────────────
-- One row per Supabase Auth user. telegram_id is filled in when the user links
-- their Telegram account.
create table public.profiles (
    id           uuid        primary key references auth.users (id) on delete cascade,
    telegram_id  bigint      unique,
    display_name text,
    created_at   timestamptz not null default now()
);

alter table public.profiles enable row level security;

-- A logged-in user can read and update only their own profile row.
create policy "profiles: read own"
    on public.profiles for select
    to authenticated
    using ((select auth.uid()) = id);

create policy "profiles: update own"
    on public.profiles for update
    to authenticated
    using ((select auth.uid()) = id)
    with check ((select auth.uid()) = id);

grant select, update on public.profiles to authenticated;

-- Auto-create a profile whenever a new auth user signs up.
create function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
    insert into public.profiles (id, display_name)
    values (new.id, coalesce(new.raw_user_meta_data ->> 'name', new.email));
    return new;
end;
$$;

create trigger on_auth_user_created
    after insert on auth.users
    for each row execute function public.handle_new_user();

-- ── telegram_link_codes ─────────────────────────────────────────────────────
-- Short-lived, single-use codes the dashboard hands out to link a Telegram
-- account to a profile. RLS is enabled with NO policies and NO grants to
-- anon/authenticated: the table is reachable only through the SECURITY DEFINER
-- RPC below (which inserts) and the bot's direct connection (which consumes).
create table public.telegram_link_codes (
    code        text        primary key,
    profile_id  uuid        not null references public.profiles (id) on delete cascade,
    created_at  timestamptz not null default now(),
    expires_at  timestamptz not null,
    consumed_at timestamptz
);

alter table public.telegram_link_codes enable row level security;

-- RPC: the authenticated dashboard user mints a fresh link code for itself.
-- SECURITY DEFINER is required so it can write to telegram_link_codes (which is
-- otherwise inaccessible); the auth.uid() guard and the targeted grant keep it
-- callable only by genuinely logged-in users.
create function public.request_telegram_link_code()
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
    v_uid  uuid := (select auth.uid());
    v_code text;
begin
    if v_uid is null then
        raise exception 'not authenticated';
    end if;

    v_code := upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 8));

    insert into public.telegram_link_codes (code, profile_id, expires_at)
    values (v_code, v_uid, now() + interval '15 minutes');

    return v_code;
end;
$$;

-- Postgres grants EXECUTE to PUBLIC by default; lock it down to authenticated.
revoke execute on function public.request_telegram_link_code() from public;
grant execute on function public.request_telegram_link_code() to authenticated;

-- ── transactions ────────────────────────────────────────────────────────────
create table public.transactions (
    id          uuid        primary key default gen_random_uuid(),
    user_id     uuid        not null references public.profiles (id) on delete cascade,
    amount      numeric     not null,
    type        text        not null check (type in ('income', 'expense')),
    category    text        not null,
    description text,
    occurred_at timestamptz not null,
    created_at  timestamptz not null default now()
);

create index transactions_occurred_at_idx on public.transactions (occurred_at desc);
create index transactions_user_id_idx on public.transactions (user_id);

alter table public.transactions enable row level security;

-- Shared household view: any logged-in family member can read all transactions.
-- TO ISOLATE PER USER instead, change USING to:
--     using ((select auth.uid()) = user_id)
create policy "transactions: family read"
    on public.transactions for select
    to authenticated
    using (true);

grant select on public.transactions to authenticated;
-- Writes come only from the bot via the direct Postgres connection (bypasses
-- RLS), so there are deliberately no insert/update/delete policies or grants.
