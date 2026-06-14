import { useQuery } from "@tanstack/react-query";
import { supabase } from "./supabase";
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
 * Fetches all transactions ordered newest-first. For a 2-person family the
 * volume is tiny, so we load everything once and derive the overview charts
 * client-side rather than running many aggregate queries.
 */
export function useTransactions() {
  return useQuery({
    queryKey: ["transactions"],
    queryFn: async (): Promise<Transaction[]> => {
      const { data, error } = await supabase
        .from("transactions")
        .select("*")
        .order("occurred_at", { ascending: false });

      if (error) throw error;
      return data ?? [];
    },
  });
}
