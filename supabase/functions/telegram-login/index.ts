// Bot-issued login code → Supabase session.
//
// The dashboard posts a one-time { token } here (delivered to the user by the
// bot via Telegram DM). We validate the token, enforce a small registration
// cap, provision (or fetch) the Supabase user for that Telegram id, and return
// a one-time email OTP. The browser then calls supabase.auth.verifyOtp() to get
// a real, refreshable Supabase session — so RLS keeps working via auth.uid().
//
// Trust model: only the bot can write to login_tokens (RLS-locked), and the bot
// only writes after Telegram has authenticated the sender. So holding a valid,
// unconsumed token proves control of that Telegram account — no HMAC needed.
import { createClient } from "jsr:@supabase/supabase-js@2";

const SUPABASE_URL = Deno.env.get("SUPABASE_URL") ?? "";
const SERVICE_ROLE_KEY = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY") ?? "";
// Max number of distinct accounts allowed to register (default 2: a family).
const MAX_PROFILES = parseInt(Deno.env.get("MAX_PROFILES") ?? "2", 10);

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "authorization, x-client-info, apikey, content-type",
  "Access-Control-Allow-Methods": "POST, OPTIONS",
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { ...corsHeaders, "Content-Type": "application/json" },
  });
}

Deno.serve(async (req) => {
  if (req.method === "OPTIONS") return new Response("ok", { headers: corsHeaders });
  if (req.method !== "POST") return json({ error: "method not allowed" }, 405);
  try {
    return await handle(req);
  } catch (e) {
    console.error("telegram-login error:", e);
    const detail =
      e instanceof Error
        ? (e.stack ?? e.message)
        : typeof e === "object" && e !== null
          ? JSON.stringify(e, Object.getOwnPropertyNames(e))
          : String(e);
    return json({ error: detail || "unknown error" }, 500);
  }
});

async function handle(req: Request): Promise<Response> {
  if (!SERVICE_ROLE_KEY) return json({ error: "server not configured (no service role key)" }, 500);

  let token: string | undefined;
  try {
    ({ token } = await req.json());
  } catch {
    return json({ error: "invalid body" }, 400);
  }
  if (!token) return json({ error: "missing token" }, 400);

  const admin = createClient(SUPABASE_URL, SERVICE_ROLE_KEY, {
    auth: { persistSession: false, autoRefreshToken: false },
  });

  // 1. Validate the one-time login token.
  const { data: row } = await admin
    .from("login_tokens")
    .select("telegram_id, telegram_username, display_name, expires_at, consumed_at")
    .eq("token", token)
    .maybeSingle();

  if (!row) return json({ error: "kode login tidak valid" }, 401);
  if (row.consumed_at) return json({ error: "kode login sudah dipakai" }, 401);
  if (new Date(row.expires_at).getTime() < Date.now()) {
    return json({ error: "kode login kadaluarsa, minta /login lagi" }, 401);
  }

  const telegramId = row.telegram_id as number;
  const email = `telegram_${telegramId}@telegram.local`;

  // 2. Enforce the registration cap for brand-new accounts.
  const { data: existing } = await admin
    .from("profiles")
    .select("id")
    .eq("telegram_id", telegramId)
    .maybeSingle();

  if (!existing) {
    const { count } = await admin
      .from("profiles")
      .select("*", { count: "exact", head: true });
    if ((count ?? 0) >= MAX_PROFILES) {
      return json({ error: "registrasi ditutup (kuota akun penuh)" }, 403);
    }
  }

  // 3. Provision/fetch the user and mint a one-time OTP (no email is sent).
  const { data, error } = await admin.auth.admin.generateLink({
    type: "magiclink",
    email,
    options: {
      data: {
        telegram_id: telegramId,
        telegram_username: row.telegram_username,
        display_name: row.display_name,
      },
    },
  });
  if (error || !data?.user || !data.properties?.email_otp) {
    console.error("generateLink failed:", JSON.stringify(error), "status=", (error as { status?: number })?.status);
    return json(
      {
        error: `generateLink gagal: ${error?.message || "(kosong)"}`,
        status: (error as { status?: number })?.status ?? null,
        code: (error as { code?: string })?.code ?? null,
      },
      500,
    );
  }

  // 4. Keep the profile fresh, then burn the token.
  await admin.from("profiles").upsert(
    {
      id: data.user.id,
      telegram_id: telegramId,
      telegram_username: row.telegram_username,
      display_name: row.display_name,
    },
    { onConflict: "id" },
  );
  await admin.from("login_tokens").update({ consumed_at: new Date().toISOString() }).eq("token", token);

  return json({ email, token: data.properties.email_otp });
}
