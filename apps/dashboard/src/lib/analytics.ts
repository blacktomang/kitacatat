import type { Transaction } from "./types";

export interface Totals {
  income: number;
  expense: number;
  net: number;
}

function monthKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/**
 * Inclusive start / exclusive end ISO timestamps for the `count`-month window
 * ending at `ref`'s month. Used to scope the overview fetch to a bounded range
 * instead of loading every transaction ever recorded.
 */
export function monthWindow(ref: Date, count = 6): { start: string; end: string } {
  const start = new Date(ref.getFullYear(), ref.getMonth() - (count - 1), 1);
  const end = new Date(ref.getFullYear(), ref.getMonth() + 1, 1);
  return { start: start.toISOString(), end: end.toISOString() };
}

/** Income/expense/net totals for the calendar month of `ref` (default now). */
export function monthlyTotals(txs: Transaction[], ref = new Date()): Totals {
  const key = monthKey(ref);
  let income = 0;
  let expense = 0;
  for (const t of txs) {
    if (monthKey(new Date(t.occurred_at)) !== key) continue;
    if (t.type === "income") income += t.amount;
    else expense += t.amount;
  }
  return { income, expense, net: income - expense };
}

export interface CategorySlice {
  category: string;
  total: number;
}

/** Expense totals grouped by category for the calendar month of `ref`. */
export function expenseByCategory(txs: Transaction[], ref = new Date()): CategorySlice[] {
  const key = monthKey(ref);
  const map = new Map<string, number>();
  for (const t of txs) {
    if (t.type !== "expense") continue;
    if (monthKey(new Date(t.occurred_at)) !== key) continue;
    map.set(t.category, (map.get(t.category) ?? 0) + t.amount);
  }
  return [...map.entries()]
    .map(([category, total]) => ({ category, total }))
    .sort((a, b) => b.total - a.total);
}

export interface MonthBar {
  month: string; // ISO first-of-month, e.g. 2025-03-01
  income: number;
  expense: number;
}

/** Income/expense per month for the last `count` months (oldest first). */
export function monthlySeries(txs: Transaction[], count = 6, ref = new Date()): MonthBar[] {
  const buckets = new Map<string, MonthBar>();
  // Seed the last `count` months so empty months still render.
  for (let i = count - 1; i >= 0; i--) {
    const d = new Date(ref.getFullYear(), ref.getMonth() - i, 1);
    buckets.set(monthKey(d), {
      month: `${monthKey(d)}-01`,
      income: 0,
      expense: 0,
    });
  }
  for (const t of txs) {
    const key = monthKey(new Date(t.occurred_at));
    const bucket = buckets.get(key);
    if (!bucket) continue;
    if (t.type === "income") bucket.income += t.amount;
    else bucket.expense += t.amount;
  }
  return [...buckets.values()];
}
