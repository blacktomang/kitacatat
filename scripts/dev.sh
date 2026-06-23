#!/usr/bin/env sh
# Local dev entrypoint: boot the Dockerized Supabase stack, point the bot and
# dashboard at it, then run both via Turbo. Requires Docker to be running.
#
# Exported env vars take precedence over .env in BOTH apps:
#   - the Go bot uses godotenv, which never overrides vars already in the env
#   - Vite exposes VITE_-prefixed process.env vars to import.meta.env
# So your .env only needs the non-Supabase secrets (TELEGRAM_BOT_TOKEN,
# GEMINI_API_KEY, VITE_TELEGRAM_BOT_USERNAME, TESSDATA_PREFIX).
set -e

if ! docker info >/dev/null 2>&1; then
  echo "✖ Docker isn't running. Start Docker Desktop (or your engine), then retry." >&2
  echo "  (To run the apps against a remote Supabase instead, use: pnpm dev:apps)" >&2
  exit 1
fi

echo "▶ Starting local Supabase (first run pulls images — can take a minute)..."
# Idempotent: starts the stack if needed and applies supabase/migrations.
supabase start

# Map the local stack's connection details onto the app env var names. The
# local stack only exposes the legacy anon key, so we feed that into the app's
# publishable-key var (interchangeable locally — both are the low-privilege,
# RLS-gated client key).
eval "$(supabase status -o env \
  --override-name api.url=VITE_SUPABASE_URL \
  --override-name auth.anon_key=VITE_SUPABASE_PUBLISHABLE_KEY \
  --override-name db.url=DATABASE_URL)"

: "${DATABASE_URL:?could not read local DB url from 'supabase status'}"
: "${VITE_SUPABASE_URL:?could not read local API url from 'supabase status'}"
: "${VITE_SUPABASE_PUBLISHABLE_KEY:?could not read local anon key from 'supabase status'}"

export DATABASE_URL VITE_SUPABASE_URL VITE_SUPABASE_PUBLISHABLE_KEY

# Serve Edge Functions in the background — the /login flow needs telegram-login,
# and `supabase start` does NOT serve functions on its own.
echo "▶ Serving Edge Functions (logs: /tmp/kitacatat-functions.log)..."
supabase functions serve --no-verify-jwt >/tmp/kitacatat-functions.log 2>&1 &
FUNCTIONS_PID=$!
trap 'kill "$FUNCTIONS_PID" 2>/dev/null' EXIT INT TERM

echo "▶ Supabase ready."
echo "    API:     $VITE_SUPABASE_URL"
echo "    DB:      $DATABASE_URL"
echo "    Studio:  http://127.0.0.1:54323"
echo "▶ Starting bot + dashboard..."

# Not exec'd, so the trap above can clean up the functions server on exit.
turbo run dev
