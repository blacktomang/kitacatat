import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { supabase } from "./supabase";
import { useAuth } from "./auth";
import type {
  Profile,
  RecurringRule,
  RecurringRuleInput,
  Transaction,
} from "./types";

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

/** All recurring rules (shared household view), oldest first. */
export function useRecurringRules() {
  return useQuery({
    queryKey: ["recurring_rules"],
    queryFn: async (): Promise<RecurringRule[]> => {
      const { data, error } = await supabase
        .from("recurring_rules")
        .select("*")
        .order("created_at", { ascending: true });

      if (error) throw error;
      return data ?? [];
    },
  });
}

/**
 * Create / update / delete recurring rules. Each mutation invalidates the
 * rules list on success; RLS enforces that a user only writes their own rows,
 * so creates must stamp user_id with the current session id.
 */
export function useCreateRecurringRule() {
  const qc = useQueryClient();
  const { session } = useAuth();
  return useMutation({
    mutationFn: async (input: RecurringRuleInput): Promise<void> => {
      const userId = session?.user.id;
      if (!userId) throw new Error("Belum masuk.");
      const { error } = await supabase
        .from("recurring_rules")
        .insert({ ...input, user_id: userId });
      if (error) throw error;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["recurring_rules"] }),
  });
}

export function useUpdateRecurringRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      id,
      patch,
    }: {
      id: string;
      patch: Partial<RecurringRuleInput>;
    }): Promise<void> => {
      const { error } = await supabase
        .from("recurring_rules")
        .update(patch)
        .eq("id", id);
      if (error) throw error;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["recurring_rules"] }),
  });
}

export function useDeleteRecurringRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string): Promise<void> => {
      const { error } = await supabase
        .from("recurring_rules")
        .delete()
        .eq("id", id);
      if (error) throw error;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["recurring_rules"] }),
  });
}
