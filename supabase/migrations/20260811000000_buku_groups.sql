-- Buku: named money-tracking groups.
--
-- Users can create "buku" (books), switch between them in the bot, and share
-- them read-only with other users. The dashboard shows a book selector to
-- filter transactions by group.
--
-- Tables:
--   groups          — one row per book, owned by a profile
--   group_members   — membership (owner or viewer)
-- Modifications:
--   transactions    — nullable group_id
--   profiles        — active_book_id (the book new transactions flow into)

-- ── groups ───────────────────────────────────────────────────────────────────
create table public.groups (
    id          uuid        primary key default gen_random_uuid(),
    owner_id    uuid        not null references public.profiles (id) on delete cascade,
    name        text        not null,
    created_at  timestamptz not null default now(),
    unique (owner_id, name)
);

alter table public.groups enable row level security;

-- Members (including the owner) can read the group details.
create policy "groups: read member"
    on public.groups for select
    to authenticated
    using (exists (
        select 1 from public.group_members gm
        where gm.group_id = id and gm.user_id = (select auth.uid())
    ));

-- Only the owner can delete a group.
create policy "groups: delete owner"
    on public.groups for delete
    to authenticated
    using (owner_id = (select auth.uid()));

grant select, insert, delete on public.groups to authenticated;

-- ── group_members ────────────────────────────────────────────────────────────
create table public.group_members (
    group_id    uuid        not null references public.groups (id) on delete cascade,
    user_id     uuid        not null references public.profiles (id) on delete cascade,
    role        text        not null check (role in ('owner', 'viewer')),
    created_at  timestamptz not null default now(),
    primary key (group_id, user_id)
);

alter table public.group_members enable row level security;

-- Owners can insert and delete members.
create policy "group_members: manage owner"
    on public.group_members for all
    to authenticated
    using (exists (
        select 1 from public.groups g
        where g.id = group_id and g.owner_id = (select auth.uid())
    ));

-- Members can read their own memberships.
create policy "group_members: read member"
    on public.group_members for select
    to authenticated
    using (user_id = (select auth.uid()));

grant select, insert, delete on public.group_members to authenticated;

-- ── Auto-insert owner as member ──────────────────────────────────────────────
create function public.handle_new_group()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
    insert into public.group_members (group_id, user_id, role)
    values (new.id, new.owner_id, 'owner');
    return new;
end;
$$;

create trigger on_group_created
    after insert on public.groups
    for each row execute function public.handle_new_group();

-- ── transactions: nullable group_id ─────────────────────────────────────────
alter table public.transactions
    add column group_id uuid references public.groups (id) on delete set null;

create index transactions_group_id_idx on public.transactions (group_id);

-- Replace per-user isolation policy with one that includes shared-book reads.
drop policy if exists "transactions: read own" on public.transactions;

create policy "transactions: read own or shared book"
    on public.transactions for select
    to authenticated
    using (
        (select auth.uid()) = user_id
        or
        (group_id is not null and exists (
            select 1 from public.group_members gm
            where gm.group_id = transactions.group_id
              and gm.user_id = (select auth.uid())
        ))
    );

-- ── profiles: active_book_id + read group members ───────────────────────────
alter table public.profiles
    add column active_book_id uuid references public.groups (id) on delete set null;

-- Extend profiles read policy so users can read the profiles of members who
-- share a group with them (needed to show member names in the dashboard).
drop policy if exists "profiles: read own" on public.profiles;

create policy "profiles: read own or shared group members"
    on public.profiles for select
    to authenticated
    using (
        id = (select auth.uid())
        or
        exists (
            select 1 from public.group_members gm1
            join public.group_members gm2 on gm1.group_id = gm2.group_id
            where gm1.user_id = profiles.id
              and gm2.user_id = (select auth.uid())
        )
    );
