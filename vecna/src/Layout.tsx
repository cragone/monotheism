import { Link, Outlet } from "react-router-dom";

export default function Layout() {
  return (
    <div className="min-h-screen bg-base-100">
      <div className="navbar bg-base-200 shadow-sm px-4">
        <Link to="/" className="text-lg font-semibold">
          Vecna
        </Link>
        <span className="ml-2 text-sm opacity-60">house control</span>
        <Link to="/settings" className="ml-auto btn btn-ghost btn-sm btn-circle" aria-label="Device settings">
          ⚙️
        </Link>
      </div>

      <main className="max-w-5xl mx-auto p-4 space-y-4">
        <Outlet />
      </main>
    </div>
  );
}
