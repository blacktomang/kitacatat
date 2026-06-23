export type TransactionType = "income" | "expense";

export const CATEGORIES = [
  "food",
  "transport",
  "bills",
  "salary",
  "shopping",
  "health",
  "entertainment",
  "other",
] as const;

export type Category = (typeof CATEGORIES)[number];

export interface Profile {
  id: string;
  telegram_id: number | null;
  telegram_username: string | null;
  display_name: string | null;
  created_at: string;
}

export interface Transaction {
  id: string;
  user_id: string;
  amount: number;
  type: TransactionType;
  category: string;
  description: string | null;
  occurred_at: string;
  created_at: string;
}
