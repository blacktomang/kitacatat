-- Recurring income/expense rules (daily / weekly / monthly / yearly).
--
-- A rule is a template ("salary 5jt on the 25th", "insurance 1.5jt every 25
-- Dec", "jajan 100rb every Monday"). A daily pg_cron job materializes each
-- rule's most recent due occurrence into a real `transactions` row, so they
-- flow through every existing chart and the transaction history with no
-- analytics changes. Each materialized transaction back-references its rule,
-- and each rule remembers the date it last posted, so re-running the job is
-- idempotent (each occurrence posts at most once).
--
-- Access model mirrors transactions but adds the dashboard's FIRST write path:
-- any logged-in family member can READ all rules (shared household view), but a
-- rule can only be created/edited/deleted by its owner. The bot continues to
-- write via the direct Postgres connection (bypasses RLS).

-- ── recurring_rules ───────────────────────────────────────────────────────
create table public.recurring_rules (
    id             uuid        primary key default gen_random_uuid(),
    user_id        uuid        not null references public.profiles (id) on delete cascade,
    amount         numeric     not null check (amount > 0),
    type           text        not null check (type in ('income', 'expense')),
    category       text        not null,
    description    text,
    frequency      text        not null check (frequency in ('daily', 'weekly', 'monthly', 'yearly')),
    -- Schedule fields. Which ones are populated depends on `frequency`, enforced
    -- by recurring_rules_schedule_ck below. day_of_month is clamped to month
    -- length by the posting job (e.g. 31 -> 28/29/30). day_of_week is Postgres
    -- `dow`: 0 = Sunday .. 6 = Saturday.
    day_of_month   int         check (day_of_month between 1 and 31),
    month_of_year  int         check (month_of_year between 1 and 12),
    day_of_week    int         check (day_of_week between 0 and 6),
    active         boolean     not null default true,
    -- Date of the most recent occurrence this rule was materialized for.
    last_posted_on date,
    created_at     timestamptz not null default now(),

    constraint recurring_rules_schedule_ck check (
        case frequency
            when 'daily'   then day_of_month is null     and month_of_year is null     and day_of_week is null
            when 'weekly'  then day_of_week  is not null  and day_of_month is null      and month_of_year is null
            when 'monthly' then day_of_month is not null  and month_of_year is null     and day_of_week is null
            when 'yearly'  then day_of_month is not null  and month_of_year is not null and day_of_week is null
        end
    )
);

create index recurring_rules_user_id_idx on public.recurring_rules (user_id);

alter table public.recurring_rules enable row level security;

-- Shared household view: any logged-in family member can read all rules.
create policy "recurring_rules: family read"
    on public.recurring_rules for select
    to authenticated
    using (true);

-- Owner-only writes — the dashboard's first authenticated write path.
create policy "recurring_rules: insert own"
    on public.recurring_rules for insert
    to authenticated
    with check ((select auth.uid()) = user_id);

create policy "recurring_rules: update own"
    on public.recurring_rules for update
    to authenticated
    using ((select auth.uid()) = user_id)
    with check ((select auth.uid()) = user_id);

create policy "recurring_rules: delete own"
    on public.recurring_rules for delete
    to authenticated
    using ((select auth.uid()) = user_id);

grant select, insert, update, delete on public.recurring_rules to authenticated;

-- ── transactions back-reference ───────────────────────────────────────────
-- Auto-posted transactions point back at the rule that generated them, so the
-- dashboard can badge them and history survives a rule being deleted.
alter table public.transactions
    add column if not exists recurring_rule_id uuid
        references public.recurring_rules (id) on delete set null;

-- ── occurrence math ───────────────────────────────────────────────────────
-- Given a rule's schedule, return the most recent occurrence date on or before
-- p_as_of. Deterministic in its inputs, so IMMUTABLE.
create function public.recurring_last_occurrence(
    p_frequency     text,
    p_day_of_month  integer,
    p_month_of_year integer,
    p_day_of_week   integer,
    p_as_of         date
) returns date
language plpgsql
immutable
set search_path = ''
as $$
declare
    v_month_start date;
    v_day         int;
    v_cand        date;
    v_year        int;
begin
    if p_frequency = 'daily' then
        return p_as_of;

    elsif p_frequency = 'weekly' then
        -- extract(dow): 0 = Sunday .. 6 = Saturday. Step back to the target dow.
        return p_as_of - ((extract(dow from p_as_of)::int - p_day_of_week + 7) % 7);

    elsif p_frequency = 'monthly' then
        v_month_start := date_trunc('month', p_as_of)::date;
        v_day := least(p_day_of_month,
                       extract(day from (v_month_start + interval '1 month - 1 day'))::int);
        v_cand := v_month_start + (v_day - 1);
        if v_cand <= p_as_of then
            return v_cand;
        end if;
        -- Recurrence day hasn't arrived this month yet; use last month's.
        v_month_start := (v_month_start - interval '1 month')::date;
        v_day := least(p_day_of_month,
                       extract(day from (v_month_start + interval '1 month - 1 day'))::int);
        return v_month_start + (v_day - 1);

    elsif p_frequency = 'yearly' then
        v_year := extract(year from p_as_of)::int;
        v_day := least(p_day_of_month,
                       extract(day from (make_date(v_year, p_month_of_year, 1) + interval '1 month - 1 day'))::int);
        v_cand := make_date(v_year, p_month_of_year, v_day);
        if v_cand <= p_as_of then
            return v_cand;
        end if;
        v_year := v_year - 1;
        v_day := least(p_day_of_month,
                       extract(day from (make_date(v_year, p_month_of_year, 1) + interval '1 month - 1 day'))::int);
        return make_date(v_year, p_month_of_year, v_day);
    end if;

    return null;
end;
$$;

-- ── materialization ───────────────────────────────────────────────────────
-- For each active rule, post a transaction for its most recent occurrence if we
-- haven't already. SECURITY DEFINER so it can write to transactions (which
-- authenticated users cannot). Returns the number of rows posted.
create function public.post_due_recurring(as_of date default current_date)
returns integer
language plpgsql
security definer
set search_path = ''
as $$
declare
    v_posted integer := 0;
    r        record;
    v_occ    date;
begin
    for r in select * from public.recurring_rules where active loop
        v_occ := public.recurring_last_occurrence(
            r.frequency, r.day_of_month, r.month_of_year, r.day_of_week, as_of);

        if v_occ is not null
           and (r.last_posted_on is null or r.last_posted_on < v_occ) then
            insert into public.transactions
                (user_id, amount, type, category, description, occurred_at, recurring_rule_id)
            values
                (r.user_id, r.amount, r.type, r.category, r.description,
                 v_occ::timestamptz, r.id);

            update public.recurring_rules
                set last_posted_on = v_occ
                where id = r.id;

            v_posted := v_posted + 1;
        end if;
    end loop;

    return v_posted;
end;
$$;

-- The job runs as the table owner; no one else should be able to invoke it.
revoke execute on function public.post_due_recurring(date) from public;

-- ── schedule ──────────────────────────────────────────────────────────────
-- Enable pg_cron and run the posting job daily at 00:05 UTC. Both steps are
-- guarded so environments without pg_cron (some local setups) still apply the
-- rest of this migration; rules can be posted manually with
--   select public.post_due_recurring();
do $$
begin
    execute 'create extension if not exists pg_cron';

    perform cron.schedule(
        'kitacatat-post-recurring',
        '5 0 * * *',
        $cron$select public.post_due_recurring();$cron$
    );
exception when others then
    raise notice 'pg_cron unavailable (%): recurring rules will not auto-post until scheduled manually', sqlerrm;
end;
$$;
