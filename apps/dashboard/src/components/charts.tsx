import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { CategorySlice, MonthBar } from "../lib/analytics";
import { formatMonth, formatRupiah } from "../lib/format";
import { Empty } from "./ui";

// Stable colors per category so the pie + legend stay consistent.
const CATEGORY_COLORS: Record<string, string> = {
  food: "#f97316",
  transport: "#3b82f6",
  bills: "#ef4444",
  salary: "#22c55e",
  shopping: "#a855f7",
  health: "#14b8a6",
  entertainment: "#ec4899",
  other: "#94a3b8",
};

function colorFor(category: string): string {
  return CATEGORY_COLORS[category] ?? "#94a3b8";
}

const compact = (n: number) =>
  new Intl.NumberFormat("id-ID", { notation: "compact" }).format(n);

export function ExpensePie({ data }: { data: CategorySlice[] }) {
  if (data.length === 0) return <Empty label="Belum ada pengeluaran bulan ini." />;

  return (
    <ResponsiveContainer width="100%" height={280}>
      <PieChart>
        <Pie
          data={data}
          dataKey="total"
          nameKey="category"
          cx="50%"
          cy="50%"
          innerRadius={60}
          outerRadius={100}
          paddingAngle={2}
        >
          {data.map((slice) => (
            <Cell key={slice.category} fill={colorFor(slice.category)} />
          ))}
        </Pie>
        <Tooltip
          formatter={(value: number, name) => [formatRupiah(value), String(name)]}
        />
        <Legend />
      </PieChart>
    </ResponsiveContainer>
  );
}

export function MonthlyBars({ data }: { data: MonthBar[] }) {
  return (
    <ResponsiveContainer width="100%" height={280}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
        <XAxis
          dataKey="month"
          tickFormatter={(m: string) => formatMonth(m)}
          tick={{ fontSize: 12, fill: "#64748b" }}
          axisLine={false}
          tickLine={false}
        />
        <YAxis
          tickFormatter={compact}
          tick={{ fontSize: 12, fill: "#64748b" }}
          axisLine={false}
          tickLine={false}
          width={48}
        />
        <Tooltip
          formatter={(value: number, name) => [formatRupiah(value), String(name)]}
          labelFormatter={(m: string) => formatMonth(m)}
        />
        <Legend />
        <Bar dataKey="income" name="Pemasukan" fill="#22c55e" radius={[4, 4, 0, 0]} />
        <Bar dataKey="expense" name="Pengeluaran" fill="#ef4444" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}
