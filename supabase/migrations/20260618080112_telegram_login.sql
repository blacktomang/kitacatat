-- Switch dashboard auth to "Sign in with Telegram" via a bot-issued login code
-- (no widget, no HTTPS domain needed — works on localhost).
--
-- Flow: user DMs /login to the bot; the bot (which Telegram has already
-- authenticated as that user) writes a short-lived, single-use token here and
-- DMs a dashboard link. The dashboard hands the token to the `telegram-login`
-- Edge Function, which validates it and mints a Supabase session. The old
-- one-time link-code flow (telegram_link_codes + request_telegram_link_code)
-- is obsolete, so we drop it.

-- Store the Telegram @username for display on the dashboard.
alter table public.profiles
    add column if not exists telegram_username text;

-- The auth user is created by the Edge Function via the admin API with the
-- Telegram details in user_metadata. Seed the profile from that metadata on
-- signup (the Edge Function also upserts these on every login to keep them
-- fresh).
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
    insert into public.profiles (id, telegram_id, telegram_username, display_name)
    values (
        new.id,
        nullif(new.raw_user_meta_data ->> 'telegram_id', '')::bigint,
        new.raw_user_meta_data ->> 'telegram_username',
        coalesce(new.raw_user_meta_data ->> 'display_name', new.email)
    )
    on conflict do nothing;  -- never let a profile clash roll back user creation
    return new;
end;
$$;

-- Short-lived, single-use login codes issued by the bot. Reachable only by the
-- bot (direct Postgres connection) and the Edge Function (service role): RLS is
-- on with no policies and no grants to anon/authenticated.
create table public.login_tokens (
    token             text        primary key,
    telegram_id       bigint      not null,
    telegram_username text,
    display_name      text,
    created_at        timestamptz not null default now(),
    expires_at        timestamptz not null,
    consumed_at       timestamptz
);

alter table public.login_tokens enable row level security;

-- Drop the now-unused one-time link-code mechanism.
drop function if exists public.request_telegram_link_code();
drop table if exists public.telegram_link_codes;
