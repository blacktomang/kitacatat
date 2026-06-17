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

# Map the local stack's connection details onto the app env var names.
eval "$(supabase status -o env \
  --override-name api.url=VITE_SUPABASE_URL \
  --override-name auth.anon_key=VITE_SUPABASE_ANON_KEY \
  --override-name db.url=DATABASE_URL)"

: "${DATABASE_URL:?could not read local DB url from 'supabase status'}"
: "${VITE_SUPABASE_URL:?could not read local API url from 'supabase status'}"
: "${VITE_SUPABASE_ANON_KEY:?could not read local anon key from 'supabase status'}"

export DATABASE_URL VITE_SUPABASE_URL VITE_SUPABASE_ANON_KEY

echo "▶ Supabase ready."
echo "    API:     $VITE_SUPABASE_URL"
echo "    DB:      $DATABASE_URL"
echo "    Studio:  http://127.0.0.1:54323"
echo "    Inbucket (magic-link emails): http://127.0.0.1:54324"
echo "▶ Starting bot + dashboard..."

exec turbo run dev
