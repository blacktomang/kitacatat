import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";

import { Card, Empty, ErrorState, Loading, SectionTitle } from "../components/ui";
import { useAuth } from "../lib/auth";
import { formatRupiah } from "../lib/format";
import {
  useCreateRecurringRule,
  useDeleteRecurringRule,
  useRecurringRules,
  useUpdateRecurringRule,
} from "../lib/hooks";
import {
  CATEGORIES,
  type Frequency,
  type RecurringRule,
  type TransactionType,
} from "../lib/types";

export const Route = createFileRoute("/recurring")({
  component: Recurring,
});

const inputClass =
  "rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm";

const FREQUENCIES: { value: Frequency; label: string }[] = [
  { value: "monthly", label: "Bulanan" },
  { value: "weekly", label: "Mingguan" },
  { value: "yearly", label: "Tahunan" },
  { value: "daily", label: "Harian" },
];

// 0 = Sunday, matching Postgres `dow` and the DB column.
const WEEKDAYS = ["Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"];
const MONTHS = [
  "Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
  "Jul", "Agu", "Sep", "Okt", "Nov", "Des",
];

/** Human-readable schedule for a stored rule, in Indonesian. */
function scheduleLabel(r: RecurringRule): string {
  switch (r.frequency) {
    case "daily":
      return "tiap hari";
    case "weekly":
      return `tiap ${WEEKDAYS[r.day_of_week ?? 0]}`;
    case "monthly":
      return `tiap tgl ${r.day_of_month}`;
    case "yearly":
      return `tiap ${r.day_of_month} ${MONTHS[(r.month_of_year ?? 1) - 1]}`;
  }
}

function Recurring() {
  const { data, isLoading, error } = useRecurringRules();

  if (isLoading) return <Loading />;
  if (error) return <ErrorState error={error} />;

  const rules = data ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Langganan</h1>
        <p className="text-sm text-slate-500">
          Pemasukan & pengeluaran rutin — otomatis tercatat tiap periode pada
          jadwal yang dipilih.
        </p>
      </div>

      <Card>
        <SectionTitle>Tambah langganan</SectionTitle>
        <AddRuleForm />
      </Card>

      <Card className="!p-0">
        {rules.length === 0 ? (
          <Empty label="Belum ada langganan." />
        ) : (
          <ul className="divide-y divide-slate-100">
            {rules.map((r) => (
              <RuleRow key={r.id} rule={r} />
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}

function AddRuleForm() {
  const create = useCreateRecurringRule();
  const [type, setType] = useState<TransactionType>("expense");
  const [amount, setAmount] = useState("");
  const [category, setCategory] = useState<string>("bills");
  const [frequency, setFrequency] = useState<Frequency>("monthly");
  const [day, setDay] = useState("1"); // day-of-month (monthly/yearly)
  const [month, setMonth] = useState("1"); // month-of-year (yearly)
  const [weekday, setWeekday] = useState("1"); // day-of-week (weekly)
  const [description, setDescription] = useState("");

  const amountNum = Number(amount);
  const dayNum = Number(day);
  const monthNum = Number(month);
  const weekdayNum = Number(weekday);

  const scheduleValid =
    frequency === "daily" ||
    (frequency === "weekly" && weekdayNum >= 0 && weekdayNum <= 6) ||
    (frequency === "monthly" && dayNum >= 1 && dayNum <= 31) ||
    (frequency === "yearly" &&
      dayNum >= 1 &&
      dayNum <= 31 &&
      monthNum >= 1 &&
      monthNum <= 12);

  const valid = Number.isFinite(amountNum) && amountNum > 0 && scheduleValid;

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid) return;
    create.mutate(
      {
        amount: amountNum,
        type,
        category,
        description: description.trim() || null,
        frequency,
        day_of_month:
          frequency === "monthly" || frequency === "yearly" ? dayNum : null,
        month_of_year: frequency === "yearly" ? monthNum : null,
        day_of_week: frequency === "weekly" ? weekdayNum : null,
        active: true,
      },
      {
        onSuccess: () => {
          setAmount("");
          setDescription("");
        },
      },
    );
  };

  return (
    <form onSubmit={submit} className="space-y-3">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Field label="Tipe">
          <select
            value={type}
            onChange={(e) => setType(e.target.value as TransactionType)}
            className={inputClass}
          >
            <option value="expense">Pengeluaran</option>
            <option value="income">Pemasukan</option>
          </select>
        </Field>

        <Field label="Jumlah (Rp)">
          <input
            type="number"
            inputMode="numeric"
            min={1}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="0"
            className={inputClass}
          />
        </Field>

        <Field label="Kategori">
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className={inputClass}
          >
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Frekuensi">
          <select
            value={frequency}
            onChange={(e) => setFrequency(e.target.value as Frequency)}
            className={inputClass}
          >
            {FREQUENCIES.map((f) => (
              <option key={f.value} value={f.value}>
                {f.label}
              </option>
            ))}
          </select>
        </Field>

        {/* Schedule inputs depend on the chosen frequency. */}
        {frequency === "monthly" ? (
          <Field label="Tiap tanggal">
            <input
              type="number"
              inputMode="numeric"
              min={1}
              max={31}
              value={day}
              onChange={(e) => setDay(e.target.value)}
              className={inputClass}
            />
          </Field>
        ) : null}

        {frequency === "yearly" ? (
          <>
            <Field label="Tanggal">
              <input
                type="number"
                inputMode="numeric"
                min={1}
                max={31}
                value={day}
                onChange={(e) => setDay(e.target.value)}
                className={inputClass}
              />
            </Field>
            <Field label="Bulan">
              <select
                value={month}
                onChange={(e) => setMonth(e.target.value)}
                className={inputClass}
              >
                {MONTHS.map((m, i) => (
                  <option key={m} value={i + 1}>
                    {m}
                  </option>
                ))}
              </select>
            </Field>
          </>
        ) : null}

        {frequency === "weekly" ? (
          <Field label="Tiap hari">
            <select
              value={weekday}
              onChange={(e) => setWeekday(e.target.value)}
              className={inputClass}
            >
              {WEEKDAYS.map((d, i) => (
                <option key={d} value={i}>
                  {d}
                </option>
              ))}
            </select>
          </Field>
        ) : null}
      </div>

      <input
        type="text"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Deskripsi (opsional), mis. gaji, sewa kos"
        className={`w-full ${inputClass}`}
      />

      {create.error ? (
        <p className="text-xs text-red-600">
          {create.error instanceof Error
            ? create.error.message
            : "Gagal menyimpan."}
        </p>
      ) : null}

      <button
        type="submit"
        disabled={!valid || create.isPending}
        className="rounded-md bg-emerald-600 px-4 py-1.5 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {create.isPending ? "Menyimpan..." : "Tambah"}
      </button>
    </form>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-slate-500">{label}</span>
      {children}
    </label>
  );
}

function RuleRow({ rule }: { rule: RecurringRule }) {
  const { session } = useAuth();
  const update = useUpdateRecurringRule();
  const del = useDeleteRecurringRule();
  const isIncome = rule.type === "income";
  // Rules are shared (family-readable) but RLS only lets the owner edit them.
  const owned = session?.user.id === rule.user_id;

  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 sm:px-5">
      <span
        className={`font-medium ${isIncome ? "text-emerald-600" : "text-red-600"} ${
          rule.active ? "" : "opacity-50"
        }`}
      >
        {isIncome ? "+" : "−"}
        {formatRupiah(rule.amount)}
      </span>
      <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
        {rule.category}
      </span>
      <span className="text-xs text-slate-500">{scheduleLabel(rule)}</span>
      {rule.description ? (
        <span className="text-sm text-slate-600">— {rule.description}</span>
      ) : null}

      <div className="ml-auto flex items-center gap-2">
        {owned ? (
          <>
            <button
              onClick={() =>
                update.mutate({ id: rule.id, patch: { active: !rule.active } })
              }
              disabled={update.isPending}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition disabled:opacity-50 ${
                rule.active
                  ? "bg-emerald-50 text-emerald-700 hover:bg-emerald-100"
                  : "bg-slate-100 text-slate-500 hover:bg-slate-200"
              }`}
            >
              {rule.active ? "Aktif" : "Nonaktif"}
            </button>
            <button
              onClick={() => {
                if (confirm("Hapus langganan ini?")) del.mutate(rule.id);
              }}
              disabled={del.isPending}
              className="rounded-md px-2.5 py-1 text-xs font-medium text-slate-400 transition hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
            >
              Hapus
            </button>
          </>
        ) : (
          <span className="text-xs text-slate-400">
            {rule.active ? "" : "nonaktif · "}milik anggota lain
          </span>
        )}
      </div>
    </li>
  );
}
