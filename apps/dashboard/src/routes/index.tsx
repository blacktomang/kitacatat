import { createFileRoute } from "@tanstack/react-router";

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

function Overview() {
  const { data, isLoading, error } = useTransactions();

  if (isLoading) return <Loading />;
  if (error) return <ErrorState error={error} />;

  const txs = data ?? [];
  const totals = monthlyTotals(txs);
  const byCategory = expenseByCategory(txs);
  const series = monthlySeries(txs);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Ringkasan bulan ini</h1>
        <p className="text-sm text-slate-500">Pemasukan & pengeluaran keluarga</p>
      </div>

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
