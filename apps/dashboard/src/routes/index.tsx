import { createFileRoute } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { BookSelector } from "../components/BookSelector";
import { ExpensePie, MonthlyBars } from "../components/charts";
import { Card, ErrorState, Loading, SectionTitle } from "../components/ui";
import {
  expenseByCategory,
  monthlySeries,
  monthlyTotals,
} from "../lib/analytics";
import { formatRupiah } from "../lib/format";
import { useTransactions } from "../lib/hooks";

export const Route = createFileRoute("/")({
  component: Overview,
});

/** Value for an `<input type="month">`, e.g. "2026-06". */
function monthValue(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function parseMonthValue(v: string): Date {
  const [year, month] = v.split("-").map(Number);
  return new Date(year, month - 1, 1);
}

function Overview() {
  const [month, setMonth] = useState(() => monthValue(new Date()));
  const [bookId, setBookId] = useState<string | null>(null);
  const monthRef = parseMonthValue(month);
  const { data, isLoading, error } = useTransactions(monthRef);

  const txs = useMemo(() => {
    if (!data) return [];
    if (!bookId) return data;
    return data.filter((t) => t.group_id === bookId);
  }, [data, bookId]);

  const totals = monthlyTotals(txs, monthRef);
  const byCategory = expenseByCategory(txs, monthRef);
  const series = monthlySeries(txs, 6, monthRef);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold tracking-tight">Ringkasan</h1>
          <p className="text-sm text-slate-500">Pemasukan & pengeluaran keluarga</p>
        </div>
        <div className="flex gap-3">
          <BookSelector selected={bookId} onChange={setBookId} />
          <label className="flex flex-col text-xs font-medium text-slate-500">
            Bulan
            <input
              type="month"
              value={month}
              max={monthValue(new Date())}
              onChange={(e) => setMonth(e.target.value)}
              className="mt-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-700 shadow-sm focus:border-slate-400 focus:outline-none"
            />
          </label>
        </div>
      </div>

      {isLoading ? (
        <Loading />
      ) : error ? (
        <ErrorState error={error} />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Stat label="Pemasukan" value={totals.income} tone="income" />
            <Stat label="Pengeluaran" value={totals.expense} tone="expense" />
            <Stat label="Sisa" value={totals.net} tone={totals.net >= 0 ? "income" : "expense"} />
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Card>
              <SectionTitle>Pengeluaran per kategori</SectionTitle>
              <ExpensePie data={byCategory} />
            </Card>
            <Card>
              <SectionTitle>Tren 6 bulan</SectionTitle>
              <MonthlyBars data={series} />
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "income" | "expense";
}) {
  const color = tone === "income" ? "text-emerald-600" : "text-red-600";
  return (
    <Card>
      <p className="text-sm text-slate-500">{label}</p>
      <p className={`mt-1 text-2xl font-bold tracking-tight ${color}`}>
        {formatRupiah(value)}
      </p>
    </Card>
  );
}
