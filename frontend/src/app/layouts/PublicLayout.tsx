import { Link, Outlet } from "react-router-dom";
import { TopNav } from "../../components/TopNav";

const publicNav = [
  { label: "About", to: "/about" },
  { label: "Privacy", to: "/privacy" },
  { label: "Login", to: "/login" }
];

export function PublicLayout(): JSX.Element {
  return (
    <div className="page-shell">
      <header className="hero-header">
        <div className="brand-row">
          <Link to="/" className="brand-link">
            Retro Save Manager
          </Link>
          <TopNav items={publicNav} />
        </div>
        <p className="hero-subtitle">Self-hosted save sync for MiSTer, RetroArch, OnionOS, OpenEmu and more.</p>
      </header>
      <main className="content-wrap">
        <Outlet />
      </main>
    </div>
  );
}
