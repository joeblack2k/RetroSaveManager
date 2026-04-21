import { Link, Outlet } from "react-router-dom";
import { TopNav } from "../../components/TopNav";

const appNav = [
  { label: "My Games", to: "/app/my-games" },
  { label: "Games", to: "/app/games" },
  { label: "Catalog", to: "/app/catalog" },
  { label: "Conflicts", to: "/app/conflicts" },
  { label: "Devices", to: "/app/devices" },
  { label: "Settings", to: "/app/settings" },
  { label: "Referrals", to: "/app/referrals" },
  { label: "Roadmap", to: "/app/roadmap" }
];

export function AppLayout(): JSX.Element {
  return (
    <div className="page-shell">
      <header className="hero-header app-header">
        <div className="brand-row">
          <Link to="/" className="brand-link">
            RetroSaveManager
          </Link>
          <TopNav items={appNav} />
        </div>
        <p className="hero-subtitle">Internal no-auth mode active for helper compatibility and LAN-only usage.</p>
      </header>
      <main className="content-wrap">
        <Outlet />
      </main>
    </div>
  );
}
