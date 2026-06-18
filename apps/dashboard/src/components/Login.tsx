import { useEffect, useRef, useState } from "react";

import { supabase } from "../lib/supabase";
import { Card } from "./ui";

const BOT_USERNAME = import.meta.env.VITE_TELEGRAM_BOT_USERNAME;
const SUPABASE_URL = import.meta.env.VITE_SUPABASE_URL;
const ANON_KEY = import.meta.env.VITE_SUPABASE_ANON_KEY;

export function Login() {
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const exchanged = useRef(false);

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token");
    if (!token || exchanged.current) return;
    exchanged.current = true;

    (async () => {
      setWorking(true);
      setError(null);
      try {
        // Exchange the bot-issued login code for a one-time OTP.
        const res = await fetch(`${SUPABASE_URL}/functions/v1/telegram-login`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            apikey: ANON_KEY,
            Authorization: `Bearer ${ANON_KEY}`,
          },
          body: JSON.stringify({ token }),
        });
        const body = await res.text();
        let data: { email?: string; token?: string; error?: string };
        try {
          data = JSON.parse(body);
        } catch {
          throw new Error(
            `Server membalas non-JSON (HTTP ${res.status}). ` +
              "Pastikan Edge Function 'telegram-login' berjalan (supabase functions serve).",
          );
        }
        if (!res.ok) throw new Error(data.error ?? "Login gagal");
        if (!data.email || !data.token) throw new Error("Respons server tidak lengkap");

        // Strip the token from the URL so a refresh can't replay it.
        window.history.replaceState({}, "", window.location.pathname);

        const { error } = await supabase.auth.verifyOtp({
          email: data.email,
          token: data.token,
          type: "email",
        });
        if (error) throw error;
        // Session set; AuthProvider's onAuthStateChange renders the app.
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setWorking(false);
      }
    })();
  }, []);

  const botLink = BOT_USERNAME ? `https://t.me/${BOT_USERNAME}` : null;

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm text-center">
        <h1 className="text-lg font-bold tracking-tight">
          kita<span className="text-emerald-600">catat</span>
        </h1>
        <p className="mt-1 mb-4 text-sm text-slate-500">
          Masuk lewat Telegram untuk melihat keuangan keluarga.
        </p>

        {working ? (
          <p className="text-sm text-slate-400">Memproses login...</p>
        ) : (
          <ol className="mb-4 list-inside list-decimal space-y-1 text-left text-sm text-slate-600">
            <li>
              Buka bot di Telegram
              {botLink && (
                <>
                  {" "}
                  (
                  <a href={botLink} target="_blank" rel="noreferrer" className="text-emerald-600 hover:underline">
                    @{BOT_USERNAME}
                  </a>
                  )
                </>
              )}
              .
            </li>
            <li>
              Kirim <code>/login</code> ke bot.
            </li>
            <li>Klik tautan masuk yang dibalas bot — kamu akan kembali ke sini dan langsung masuk.</li>
          </ol>
        )}

        {error && <p className="mt-2 text-xs text-red-600">{error}</p>}
      </Card>
    </div>
  );
}
