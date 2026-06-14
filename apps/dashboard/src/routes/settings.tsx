import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";

import { Card, ErrorState, Loading, SectionTitle } from "../components/ui";
import { useProfile } from "../lib/hooks";
import { supabase } from "../lib/supabase";

export const Route = createFileRoute("/settings")({
  component: Settings,
});

const BOT_USERNAME = import.meta.env.VITE_TELEGRAM_BOT_USERNAME;

function Settings() {
  const { data: profile, isLoading, error } = useProfile();
  const qc = useQueryClient();

  const [code, setCode] = useState<string | null>(null);
  const [genError, setGenError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  if (isLoading) return <Loading />;
  if (error) return <ErrorState error={error} />;

  const linked = profile?.telegram_id != null;
  const deepLink =
    code && BOT_USERNAME ? `https://t.me/${BOT_USERNAME}?start=${code}` : null;

  async function generate() {
    setGenerating(true);
    setGenError(null);
    const { data, error } = await supabase.rpc("request_telegram_link_code");
    setGenerating(false);
    if (error) {
      setGenError(error.message);
      return;
    }
    setCode(data as string);
  }

  return (
    <div className="max-w-xl space-y-4">
      <h1 className="text-xl font-bold tracking-tight">Hubungkan Telegram</h1>

      <Card>
        <SectionTitle>Status</SectionTitle>
        {linked ? (
          <p className="text-sm text-emerald-700">
            ✅ Terhubung dengan Telegram ID{" "}
            <span className="font-mono">{profile?.telegram_id}</span>.
          </p>
        ) : (
          <p className="text-sm text-slate-600">
            Belum terhubung. Buat kode tautan lalu kirim ke bot untuk mulai
            mencatat dari Telegram.
          </p>
        )}
      </Card>

      <Card>
        <SectionTitle>{linked ? "Tautkan ulang" : "Langkah menautkan"}</SectionTitle>
        <ol className="mb-4 list-inside list-decimal space-y-1 text-sm text-slate-600">
          <li>Tekan tombol di bawah untuk membuat kode tautan (berlaku 15 menit).</li>
          <li>Buka tautan Telegram yang muncul — ini mengirim <code>/start</code> berisi kode ke bot.</li>
          <li>Bot akan mengonfirmasi, lalu tekan "Segarkan status".</li>
        </ol>

        <button
          onClick={generate}
          disabled={generating}
          className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50"
        >
          {generating ? "Membuat..." : "Buat kode tautan"}
        </button>
        {genError && <p className="mt-2 text-xs text-red-600">{genError}</p>}

        {code && (
          <div className="mt-4 space-y-2 rounded-md bg-slate-50 p-3">
            <p className="text-sm text-slate-600">
              Kode: <span className="font-mono font-semibold">{code}</span>
            </p>
            {deepLink ? (
              <a
                href={deepLink}
                target="_blank"
                rel="noreferrer"
                className="inline-block rounded-md bg-sky-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-sky-700"
              >
                Buka di Telegram
              </a>
            ) : (
              <p className="text-xs text-amber-600">
                Set <code>VITE_TELEGRAM_BOT_USERNAME</code> untuk tombol tautan
                langsung. Sementara, kirim <code>/start {code}</code> ke bot
                secara manual.
              </p>
            )}
            <button
              onClick={() => qc.invalidateQueries({ queryKey: ["profile"] })}
              className="block text-xs font-medium text-emerald-700 hover:underline"
            >
              Saya sudah menautkan — segarkan status
            </button>
          </div>
        )}
      </Card>
    </div>
  );
}
