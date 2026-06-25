-- Isolate transactions per user.
--
-- The baseline schema shipped a "shared household" read policy
-- (`transactions: family read`, USING (true)) that let ANY authenticated user
-- read EVERY user's transactions. The dashboard reads with a bare
-- `select * from transactions`, relying entirely on RLS to scope rows, so that
-- policy is what exposed other accounts' data. Replace it with an owner-only
-- read so each logged-in user sees only their own rows.
--
-- Writes are unchanged: they still come only from the bot via the direct
-- Postgres connection (bypasses RLS), so there remain no insert/update/delete
-- policies for authenticated.

drop policy if exists "transactions: family read" on public.transactions;

create policy "transactions: read own"
    on public.transactions for select
    to authenticated
    using ((select auth.uid()) = user_id);
