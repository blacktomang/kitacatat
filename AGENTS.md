# Memory

## Project Overview
See @README.md for project overview and @package.json for available npm/pnpm commands for this project.

## Architecture

**Monorepo** (Turborepo + pnpm workspaces) with two apps and one shared config package:

```
apps/bot/          Go 1.24 — Telegram bot, OCR, AI parsing, Postgres writes
apps/dashboard/    React 19 + Vite — read-only finance dashboard
packages/config/   Shared tsconfig + eslint for the JS side
supabase/          Migrations (schema source of truth) + Edge Function (telegram-login)
```

### Bot (`apps/bot/`)

Layered Go architecture with dependency injection. Composition root in `cmd/bot/main.go`:

- **`internal/config`** — typed config from env via godotenv
- **`internal/domain`** — `Transaction` value object, `Type`/`Category` closed enums, validation. No DB or AI deps.
- **`internal/ai`** — provider-agnostic `Parser` interface. Two backends: `gemini` (native SDK, strict JSON schema) and `openai` (raw HTTP, any OpenAI-compatible endpoint). Selected by `AI_PROVIDER` env var.
- **`internal/ocr`** — Tesseract wrapper via gosseract (CGO). Extracts text from receipt images.
- **`internal/store`** — sqlc-generated queries + hand-written `Store` wrapper. Uses pgx/v5 pool.
- **`internal/telegram`** — message handlers (text, photo), `requireLinked` middleware (profile lookup by Telegram ID), `/login` flow.

**Message pipeline:** Text or photo → OCR (if image) → AI parse → Normalize/Validate → Save to Postgres → Reply with formatted Rupiah confirmation.

**Auth flow:** User DMs `/login` → bot generates token (24-byte hex, 5-min TTL) → stores in `login_tokens` → DMs dashboard link with `?token=...`.

### Dashboard (`apps/dashboard/`)

- **Routing:** TanStack Router (file-based, auto-generated `routeTree.gen.ts`). Two routes: `/` (Overview) and `/transactions` (Transaction list).
- **Data fetching:** Supabase PostgREST via TanStack Query. Custom hooks: `useProfile()`, `useTransactions(monthRef?)`. 6-month rolling window.
- **Auth:** Supabase Auth. Telegram-native login via Edge Function. Session managed via `AuthProvider` context.
- **Styling:** Tailwind CSS v4 (Vite plugin, no config file). Emerald brand color. Hand-rolled UI primitives in `components/ui.tsx`. Light-only mode.
- **Charts:** Recharts (donut for expense-by-category, bar for monthly income/expense).
- **Locale:** Bahasa Indonesia throughout. IDR currency formatting.

### Database (Supabase Postgres)

Three tables (schema defined in `supabase/migrations/`):

| Table | Purpose |
|---|---|
| `profiles` | User profiles. `id` UUID FK→auth.users, `telegram_id` (unique), `telegram_username`, `display_name` |
| `transactions` | Financial entries. `user_id` FK→profiles, `amount`, `type` (income/expense), `category`, `description`, `occurred_at` |
| `login_tokens` | Short-lived single-use login tokens. `telegram_id`, `expires_at`, `consumed_at` |

**RLS:** Profiles — own-row only. Transactions — per-user isolation (`auth.uid() = user_id`). Bot writes via direct PG connection (bypasses RLS).

**Access model:** No hardcoded user IDs. Auth via Telegram login. Registration capped at `MAX_PROFILES` (default 2).

### Edge Function (`supabase/functions/telegram-login/`)

Verifies one-time token from `login_tokens`, provisions Supabase user with synthetic email `telegram_{id}@telegram.local`, upserts profile, burns token, returns OTP for session exchange.

### CI/CD (`.github/workflows/`)

| Workflow | Trigger | Deploys |
|---|---|---|
| `ci.yml` | push/PR to main | build + lint |
| `supabase.yml` | push to main (supabase/**) | migrations + Edge Function |
| `dashboard.yml` | push to main (apps/dashboard/**) | Cloudflare Pages |
| `bot.yml` | push to main (apps/bot/**) | Docker → GHCR → VPS via SSH |

## Code Style Guidelines
- Use descriptive variable names
- Follow existing patterns in the codebase
- Extract complex conditions into meaningful boolean variables
- Bot: Go conventions, clean layer separation, domain isolation from infra
- Dashboard: React 19, TypeScript strict, Tailwind utility classes, no component library
- All UI text in Bahasa Indonesia

## Common Workflows

```bash
pnpm dev                    # Full local dev (Supabase Docker + bot + dashboard)
pnpm dev:apps               # Apps only (against hosted Supabase)
pnpm build                  # Build all (bot binary + dashboard dist)
pnpm lint                   # Lint all
pnpm db:push                # Apply migrations
pnpm db:new <name>          # Create new migration
pnpm db:reset               # Reset local DB (Docker)
pnpm --filter @kitacatat/bot run sqlc    # Regenerate sqlc code after query changes
```

## Prerequisites
- Go 1.24+, Node 20+, pnpm 9+
- Tesseract with ind+eng trained data (for local OCR)
- sqlc, Supabase CLI, Docker

## Environment Variables
See `.env.example`. Key vars: `TELEGRAM_BOT_TOKEN`, `AI_PROVIDER`/`AI_MODEL`/`AI_API_KEY`, `DATABASE_URL`, `VITE_SUPABASE_URL`, `VITE_SUPABASE_PUBLISHABLE_KEY`, `VITE_TELEGRAM_BOT_USERNAME`, `TESSDATA_PREFIX`.

## Notable Issues
- Dashboard `routeTree.gen.ts` imports a `./routes/recurring` route that no longer exists on disk. Running `tsr generate` would regenerate cleanly but the committed file is stale.
- No tests in the codebase (no test files, configs, or deps).
