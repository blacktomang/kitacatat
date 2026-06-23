import { useQuery } from "@tanstack/react-query";
import { supabase } from "./supabase";
import { monthWindow } from "./analytics";
import { useAuth } from "./auth";
import type { Profile, Transaction } from "./types";

/** The current user's profile row, including Telegram link status. */
export function useProfile() {
  const { session } = useAuth();
  const userId = session?.user.id;
  return useQuery({
    queryKey: ["profile", userId],
    enabled: !!userId,
    queryFn: async (): Promise<Profile | null> => {
      const { data, error } = await supabase
        .from("profiles")
        .select("*")
        .eq("id", userId!)
        .maybeSingle();
      if (error) throw error;
      return data;
    },
  });
}

/**
 * Fetches transactions newest-first, scoped to the 6-month window ending at
 * `monthRef` (default current month). This bounds the fetch — the selected
 * month powers the totals and category breakdown, while the full window feeds
 * the 6-month trend — instead of loading every transaction ever recorded.
 */
export function useTransactions(monthRef: Date = new Date()) {
  const { start, end } = monthWindow(monthRef, 6);
  return useQuery({
    queryKey: ["transactions", start, end],
    queryFn: async (): Promise<Transaction[]> => {
      const { data, error } = await supabase
        .from("transactions")
        .select("*")
        .gte("occurred_at", start)
        .lt("occurred_at", end)
        .order("occurred_at", { ascending: false });

      if (error) throw error;
      return data ?? [];
    },
  });
}
