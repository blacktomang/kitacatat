# kitacatat 💸

A personal finance tracker for family use (2 users). Send your spending to a
**Telegram bot** as casual text (`makan siang 50rb`, `gaji 5jt`) or a **photo**
of a receipt / bank-transfer / e-wallet screenshot. A Go bot OCRs images, asks
**Gemini** to structure the text into transaction(s), saves them to **Postgres
(Supabase)**, and a **React dashboard** visualizes everything.

```
Telegram message ─▶ Go bot ─▶ (OCR if image) ─▶ Gemini (structured JSON)
                                                     │
                                                     ▼
                                            Postgres (Supabase)
                                                     │
                                                     ▼
                                       React dashboard (reads via RLS)
```

## Repo layout

```
apps/
  bot/               # Go: Telegram bot + OCR + Gemini + Postgres
    cmd/bot/         #   entrypoint
    internal/
      config/        #   typed config from env (.env via godotenv)
      domain/        #   Transaction, Category/Type enums, validation
      ocr/           #   gosseract (Tesseract) wrapper: bytes -> text
      ai/            #   provider-agnostic Parser: text -> []Transaction (gemini | openai)
      store/         #   sqlc-generated queries + pgx pool wrapper
      telegram/      #   text / photo / image-document handlers
    db/queries/      #   sqlc query definitions
  dashboard/         # React 19 + Vite + TanStack Router/Query + Tailwind v4 + Recharts
supabase/
  config.toml        # Supabase CLI project config
  migrations/        # SQL migrations (schema source of truth, applied via `supabase db push`)
packages/
  config/            # shared tsconfig + eslint for the JS side
turbo.json           # build / dev / lint pipeline
pnpm-workspace.yaml
```

## 🔑 Accounts & keys you must supply

Put all of these in a `.env` file at the repo root (copy from `.env.example`):

| Variable | Where to get it |
| --- | --- |
| `TELEGRAM_BOT_TOKEN` | Message **@BotFather** on Telegram → `/newbot` → copy the token. |
| `AI_PROVIDER` / `AI_MODEL` / `AI_API_KEY` | The LLM provider (`gemini` or `openai`-compatible), model id, and key. See [Choosing an AI provider](#choosing-an-ai-provider). |
| `DATABASE_URL` | **Supabase** → create a project → Project Settings → Database → *Connection string (URI)*. Include `?sslmode=require`. |
| `VITE_SUPABASE_URL` | Supabase → Project Settings → API → *Project URL*. |
| `VITE_SUPABASE_ANON_KEY` | Supabase → Project Settings → API → *anon / public* key. |
| `VITE_TELEGRAM_BOT_USERNAME` | Your bot's username (without `@`) from BotFather — used to build the Telegram deep link on the linking page. |
| `TESSDATA_PREFIX` | Path to Tesseract trained data (see install step). macOS Homebrew: `/opt/homebrew/share/tessdata`. |

> **No hardcoded user IDs.** Access is granted by linking a Telegram account to
> a logged-in dashboard profile (see [Login & linking](#login--linking)). The
> bot writes to Postgres directly with `DATABASE_URL` (bypasses RLS); the
> dashboard reads with the anon key gated by Row Level Security.

## Prerequisites

- **Go** 1.24+ — `brew install go`
- **pnpm** + **Node** 20+ — `brew install pnpm node`
- **Tesseract** with **Indonesian + English** trained data (for local OCR):
  - macOS: `brew install tesseract tesseract-lang`
    (installs `ind`/`eng` into `/opt/homebrew/share/tessdata`)
  - Debian/Ubuntu: `sudo apt-get install tesseract-ocr tesseract-ocr-ind tesseract-ocr-eng libtesseract-dev libleptonica-dev`
- **sqlc** (generate DB code) — `brew install sqlc`
- **Supabase CLI** (migrations + local stack) — `brew install supabase/tap/supabase`
- **Docker** (for the local Supabase stack used by `pnpm dev`) — Docker Desktop / OrbStack / colima

> The bot uses CGO to link Tesseract. On macOS Homebrew, Leptonica is keg-only;
> the bot's npm scripts (`apps/bot/scripts/go.sh`) set the right `CGO_*` paths
> automatically, so `pnpm dev` / `turbo build` just work. On Linux the apt
> headers are already on the default path.

## Setup

```bash
# 1. Install JS deps (Turbo, dashboard deps, shared config)
pnpm install

# 2. Configure
cp .env.example .env
$EDITOR .env          # fill in the keys from the table above

# 3. Schema.
#    LOCAL DEV: skip this — `pnpm dev` boots a local Supabase stack and applies
#    supabase/migrations automatically.
#    HOSTED (shared/prod) project: link it once and push the migrations.
#    Find <project-ref> in your project's URL or Project Settings → General.
supabase login                       # one-time, opens a browser
supabase link --project-ref <project-ref>
supabase db push                     # applies supabase/migrations/*  (= pnpm db:push)

# 4. (Re)generate type-safe DB code from db/queries — only needed if you change
#    the schema (supabase/migrations) or queries; generated code is committed.
cd apps/bot && sqlc generate && cd -
#    …or: pnpm --filter @kitacatat/bot run sqlc

#    To add a new migration later:  pnpm db:new <name>   (then edit the file,
#    then pnpm db:push). To iterate locally with Docker: supabase db reset.
```

## Run

```bash
# Local dev: boots the Dockerized Supabase stack, points both apps at it,
# then runs bot + dashboard. (Requires Docker running.)
pnpm dev

# Run only the apps against whatever your env points to (e.g. a hosted
# Supabase) without starting the local stack:
pnpm dev:apps

# Or individually:
pnpm --filter @kitacatat/bot run dev          # starts the Telegram bot
pnpm --filter @kitacatat/dashboard run dev    # Vite dev server (http://localhost:5173)
```

**What `pnpm dev` does:** runs `supabase start` (applies `supabase/migrations`
to a local Postgres), then exports the local stack's connection details onto
`DATABASE_URL` / `VITE_SUPABASE_URL` / `VITE_SUPABASE_ANON_KEY` before launching
Turbo. These exported vars override `.env`, so locally your `.env` only needs the
non-Supabase secrets: `TELEGRAM_BOT_TOKEN`, `GEMINI_API_KEY`,
`VITE_TELEGRAM_BOT_USERNAME`, and `TESSDATA_PREFIX`.

Handy local URLs (from `supabase start`):

- Studio (DB UI): <http://127.0.0.1:54323>
- **Inbucket** (catches magic-link login emails locally): <http://127.0.0.1:54324>

> Logging in locally won't send a real email — open Inbucket to click the magic
> link. Stop the stack with `pnpm supabase:stop`.

### Login & linking

Before the bot will record anything, each family member must connect their
Telegram account once:

1. Open the dashboard → enter your email → click the **magic link** Supabase
   emails you (passwordless login).
2. Go to **Hubungkan Telegram** → **Buat kode tautan** (creates a single-use,
   15-minute code via a `SECURITY DEFINER` RPC).
3. Click **Buka di Telegram** — this sends `/start <code>` to the bot, which
   links your Telegram id to your profile. Back on the page, click **segarkan
   status** to confirm.

After that, the bot serves you (and rejects anyone unlinked). In Supabase, add
both family members' emails under **Authentication → Users** (or leave signups
on, then turn them off).

Then DM your bot on Telegram:

- `makan siang 50rb` → 1 expense, category *food*
- `gaji 5jt` → 1 income, category *salary*
- a photo of a receipt → OCR → the **total** is captured (add a caption for
  extra context, e.g. "belanja bulanan")
- an image sent **as a file** (document) → handled the same way

## Build

```bash
pnpm build        # = turbo run build
# bot     -> apps/bot/bin/bot
# dashboard -> apps/dashboard/dist
```

### Bot Docker image (OCR runs inside the container)

```bash
cd apps/bot
docker build -t kitacatat-bot .
docker run --rm --env-file ../../.env kitacatat-bot
```

The image is multi-stage: it builds with the Tesseract/Leptonica dev headers
(`CGO_ENABLED=1`) and ships a slim runtime that installs the Tesseract runtime
plus `tesseract-ocr-ind` + `tesseract-ocr-eng` and sets `TESSDATA_PREFIX`.

## Data model & Row Level Security

The baseline migration in `supabase/migrations/` creates:

- **`profiles`** — one row per Supabase Auth user (auto-created by a trigger on
  `auth.users`), with a unique `telegram_id` filled in at link time.
- **`telegram_link_codes`** — single-use, 15-minute codes. Created only via the
  `request_telegram_link_code()` RPC (`SECURITY DEFINER`, locked to the
  `authenticated` role); consumed by the bot.
- **`transactions.user_id`** is a `uuid` FK to `profiles(id)` — the bot looks up
  the profile by `telegram_id` and stamps each transaction with it.

RLS (all policies scoped with `TO authenticated`):

- `profiles`: a user can only read/update their own row (`auth.uid() = id`;
  the update policy also has a matching `WITH CHECK`).
- `transactions`: any logged-in family member can **read all** rows (`USING
  (true)`) — a shared household view. To isolate per user instead, change the
  policy to `USING ((select auth.uid()) = user_id)` (noted inline in the
  migration).
- The bot writes via the direct Postgres connection, which bypasses RLS, so
  there are no write policies/grants for `anon`/`authenticated`.

## Choosing an AI provider

`internal/ai` is **provider-agnostic** — a `Parser` interface with two
implementations, selected by env:

- **`AI_PROVIDER=gemini`** (`gemini.go`) — Google's native SDK with a strict
  response schema. Default model `gemini-2.5-flash`.
- **`AI_PROVIDER=openai`** (`openai.go`) — any **OpenAI-compatible**
  `/chat/completions` endpoint. Switch providers by just setting `AI_BASE_URL` +
  `AI_MODEL` + `AI_API_KEY` — no code changes. Works with OpenAI, Groq,
  OpenRouter, DeepSeek, Together, local Ollama, …

This app's calls are tiny (a short message or OCR text in, small JSON out), so
cost is fractions of a cent per message — pick for **reliability**, not price:

| Want | Set |
| --- | --- |
| Cheapest reliable Gemini | `AI_PROVIDER=gemini`, `AI_MODEL=gemini-2.5-flash-lite` (enable billing to avoid free-tier "high demand" 503s) |
| Free + fast | `AI_PROVIDER=openai`, `AI_BASE_URL=https://api.groq.com/openai/v1`, a Groq Llama model |
| One key, many models | `AI_PROVIDER=openai`, `AI_BASE_URL=https://openrouter.ai/api/v1`, an OpenRouter model |

> `gemini-2.0-flash` is retired (0 free quota) — don't use it.

## How parsing works

`internal/ai` sends the text (plus any photo caption) to the configured model in
**JSON mode**. The model is told that amounts are Indonesian Rupiah (`Rp50.000`,
`50.000`, `50rb`, `5jt`, `1.250.000`); to extract **one transaction per line item**
on an itemized receipt (each with its own AI-inferred `category`, skipping the
grand total to avoid double-counting) or a single transaction for a plain
note/total; to use a date from the text (else now), and to infer `type` and
`category`. The
Gemini provider additionally enforces a strict response schema. Every field is
then **validated in Go** (`amount > 0`, `type`/`category` in the allowed sets)
and invalid items are dropped before saving.
