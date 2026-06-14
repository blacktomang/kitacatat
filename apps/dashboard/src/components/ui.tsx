import type { ReactNode } from "react";

export function Card({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5 ${className}`}
    >
      {children}
    </div>
  );
}

export function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-3 text-sm font-semibold text-slate-500">{children}</h2>;
}

export function Loading({ label = "Memuat..." }: { label?: string }) {
  return (
    <div className="flex items-center justify-center py-16 text-sm text-slate-400">
      {label}
    </div>
  );
}

export function ErrorState({ error }: { error: unknown }) {
  const message = error instanceof Error ? error.message : String(error);
  return (
    <Card className="border-red-200 bg-red-50">
      <p className="text-sm font-medium text-red-700">Gagal memuat data</p>
      <p className="mt-1 text-xs text-red-600">{message}</p>
    </Card>
  );
}

export function Empty({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center py-12 text-sm text-slate-400">
      {label}
    </div>
  );
}
