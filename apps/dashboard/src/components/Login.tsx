import { useState, type FormEvent } from "react";

import { supabase } from "../lib/supabase";
import { Card } from "./ui";

export function Login() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    const { error } = await supabase.auth.signInWithOtp({
      email,
      options: { emailRedirectTo: window.location.origin },
    });
    setLoading(false);
    if (error) setError(error.message);
    else setSent(true);
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <h1 className="text-lg font-bold tracking-tight">
          kita<span className="text-emerald-600">catat</span>
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Masuk untuk melihat keuangan keluarga.
        </p>

        {sent ? (
          <p className="mt-4 rounded-md bg-emerald-50 p-3 text-sm text-emerald-700">
            Tautan masuk sudah dikirim ke <strong>{email}</strong>. Cek emailmu
            dan klik tautannya.
          </p>
        ) : (
          <form onSubmit={submit} className="mt-4 space-y-3">
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="email@contoh.com"
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
            />
            {error && <p className="text-xs text-red-600">{error}</p>}
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50"
            >
              {loading ? "Mengirim..." : "Kirim tautan masuk"}
            </button>
          </form>
        )}
      </Card>
    </div>
  );
}
