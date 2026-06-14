import { Link, Outlet, createRootRoute } from "@tanstack/react-router";

import { Login } from "../components/Login";
import { Loading } from "../components/ui";
import { signOut, useAuth } from "../lib/auth";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  const { session, loading } = useAuth();

  if (loading) return <Loading />;
  if (!session) return <Login />;

  return (
    <div className="min-h-screen text-slate-900">
      <header className="sticky top-0 z-10 border-b border-slate-200 bg-white/80 backdrop-blur">
        <nav className="mx-auto flex max-w-5xl items-center gap-1 px-4 py-3 sm:gap-4">
          <span className="mr-2 text-lg font-bold tracking-tight">
            kita<span className="text-emerald-600">catat</span>
          </span>
          <NavLink to="/">Overview</NavLink>
          <NavLink to="/transactions">Transaksi</NavLink>
          <NavLink to="/settings">Hubungkan Telegram</NavLink>
          <button
            onClick={() => signOut()}
            className="ml-auto rounded-md px-3 py-1.5 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-slate-900"
          >
            Keluar
          </button>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}

function NavLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      className="rounded-md px-3 py-1.5 text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-900"
      activeProps={{ className: "bg-emerald-50 text-emerald-700 hover:bg-emerald-50" }}
      activeOptions={{ exact: to === "/" }}
    >
      {children}
    </Link>
  );
}
