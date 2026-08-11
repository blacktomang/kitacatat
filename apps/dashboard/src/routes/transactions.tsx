import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";

import { BookSelector } from "../components/BookSelector";
import { Card, Empty, ErrorState, Loading } from "../components/ui";
import { formatDate, formatRupiah } from "../lib/format";
import { useTransactions } from "../lib/hooks";
import { CATEGORIES, type Transaction, type TransactionType } from "../lib/types";

export const Route = createFileRoute("/transactions")({
  component: Transactions,
});

type SortKey = "occurred_at" | "amount";
type SortDir = "asc" | "desc";

function Transactions() {
  const { data, isLoading, error } = useTransactions();

  const [type, setType] = useState<TransactionType | "all">("all");
  const [category, setCategory] = useState<string>("all");
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("occurred_at");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const [bookId, setBookId] = useState<string | null>(null);

  const rows = useMemo(() => {
    let txs = (data ?? []).filter((t) => {
      if (bookId && t.group_id !== bookId) return false;
      if (type !== "all" && t.type !== type) return false;
      if (category !== "all" && t.category !== category) return false;
      if (search && !(t.description ?? "").toLowerCase().includes(search.toLowerCase()))
        return false;
      return true;
    });
    txs = [...txs].sort((a, b) => {
      const cmp =
        sortKey === "amount"
          ? a.amount - b.amount
          : a.occurred_at.localeCompare(b.occurred_at);
      return sortDir === "asc" ? cmp : -cmp;
    });
    return txs;
  }, [data, type, category, search, sortKey, sortDir, bookId]);

  if (isLoading) return <Loading />;
  if (error) return <ErrorState error={error} />;

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold tracking-tight">Transaksi</h1>

      <div className="flex flex-wrap gap-2">
        <BookSelector selected={bookId} onChange={setBookId} />

        <select
          value={type}
          onChange={(e) => setType(e.target.value as TransactionType | "all")}
          className="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm"
        >
          <option value="all">Semua tipe</option>
          <option value="income">Pemasukan</option>
          <option value="expense">Pengeluaran</option>
        </select>

        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm"
        >
          <option value="all">Semua kategori</option>
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>

        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Cari deskripsi..."
          className="flex-1 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm"
        />
      </div>

      <Card className="overflow-hidden !p-0">
        {rows.length === 0 ? (
          <Empty label="Tidak ada transaksi." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
                  <Th onClick={() => toggleSort("occurred_at")} active={sortKey === "occurred_at"} dir={sortDir}>
                    Tanggal
                  </Th>
                  <th className="px-4 py-2.5 font-medium">Deskripsi</th>
                  <th className="px-4 py-2.5 font-medium">Kategori</th>
                  <th className="px-4 py-2.5 font-medium">Tipe</th>
                  <Th onClick={() => toggleSort("amount")} active={sortKey === "amount"} dir={sortDir} align="right">
                    Jumlah
                  </Th>
                </tr>
              </thead>
              <tbody>
                {rows.map((t) => (
                  <Row key={t.id} t={t} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}

function Th({
  children,
  onClick,
  active,
  dir,
  align = "left",
}: {
  children: React.ReactNode;
  onClick: () => void;
  active: boolean;
  dir: SortDir;
  align?: "left" | "right";
}) {
  return (
    <th className={`px-4 py-2.5 font-medium ${align === "right" ? "text-right" : ""}`}>
      <button
        onClick={onClick}
        className="inline-flex items-center gap-1 hover:text-slate-900"
      >
        {children}
        <span className="text-[10px]">{active ? (dir === "asc" ? "▲" : "▼") : "↕"}</span>
      </button>
    </th>
  );
}

function Row({ t }: { t: Transaction }) {
  const isIncome = t.type === "income";
  return (
    <tr className="border-b border-slate-100 last:border-0 hover:bg-slate-50">
      <td className="whitespace-nowrap px-4 py-2.5 text-slate-600">{formatDate(t.occurred_at)}</td>
      <td className="px-4 py-2.5">{t.description || <span className="text-slate-400">—</span>}</td>
      <td className="px-4 py-2.5">
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
          {t.category}
        </span>
      </td>
      <td className="px-4 py-2.5">
        <span className={isIncome ? "text-emerald-600" : "text-red-600"}>
          {isIncome ? "Pemasukan" : "Pengeluaran"}
        </span>
      </td>
      <td className={`whitespace-nowrap px-4 py-2.5 text-right font-medium ${isIncome ? "text-emerald-600" : "text-red-600"}`}>
        {isIncome ? "+" : "−"}
        {formatRupiah(t.amount)}
      </td>
    </tr>
  );
}
