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
  // Set when this row was auto-posted from a recurring rule.
  recurring_rule_id: string | null;
}

export type Frequency = "daily" | "weekly" | "monthly" | "yearly";

export interface RecurringRule {
  id: string;
  user_id: string;
  amount: number;
  type: TransactionType;
  category: string;
  description: string | null;
  frequency: Frequency;
  // Which schedule fields are set depends on `frequency`:
  //   daily → none · weekly → day_of_week (0=Sun) · monthly → day_of_month
  //   yearly → day_of_month + month_of_year
  day_of_month: number | null;
  month_of_year: number | null;
  day_of_week: number | null;
  active: boolean;
  last_posted_on: string | null;
  created_at: string;
}

/** Fields the dashboard supplies when creating/editing a recurring rule. */
export interface RecurringRuleInput {
  amount: number;
  type: TransactionType;
  category: string;
  description: string | null;
  frequency: Frequency;
  day_of_month: number | null;
  month_of_year: number | null;
  day_of_week: number | null;
  active: boolean;
}
