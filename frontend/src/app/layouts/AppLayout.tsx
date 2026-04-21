import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { clearFrontendAuthSession, isFrontendAuthRequired } from "../../services/authSession";

type SidebarIconName = "saves" | "games" | "devices" | "settings" | "guide" | "download" | "logout";

const appNav: Array<{ label: string; to: string; icon: SidebarIconName }> = [
  { label: "My Saves", to: "/app/my-games", icon: "saves" },
  { label: "My Games", to: "/app/games", icon: "games" },
  { label: "Devices", to: "/app/devices", icon: "devices" },
  { label: "Settings", to: "/app/settings", icon: "settings" },
  { label: "Getting Started", to: "/app/getting-started", icon: "guide" },
  { label: "Download", to: "/app/download", icon: "download" }
];

export function AppLayout(): JSX.Element {
  const navigate = useNavigate();
  const authRequired = isFrontendAuthRequired();

  function handleLogout(): void {
    clearFrontendAuthSession();
    navigate("/login", { replace: true });
  }

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <Link to="/" className="sidebar-brand">
          <span className="sidebar-brand__glyph">1</span>
          <span>1Retro</span>
        </Link>

        <nav className="side-nav" aria-label="App">
          {appNav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => (isActive ? "side-nav__link side-nav__link--active" : "side-nav__link")}
            >
              <SidebarIcon name={item.icon} />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="side-nav__spacer" />

        <footer className="sidebar-user">
          <div className="sidebar-user__meta">
            <strong>j2k</strong>
            <p>{authRequired ? "Authenticated" : "Free"}</p>
          </div>
          {authRequired ? (
            <button className="sidebar-logout" type="button" aria-label="Logout" onClick={handleLogout}>
              <SidebarIcon name="logout" />
            </button>
          ) : null}
        </footer>
      </aside>

      <main className="app-main">
        <Outlet />
      </main>
    </div>
  );
}

function SidebarIcon({ name }: { name: SidebarIconName }): JSX.Element {
  switch (name) {
    case "saves":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3.5" y="7" width="17" height="11" rx="3" />
          <circle cx="9" cy="12.5" r="1.2" />
          <circle cx="15" cy="12.5" r="1.2" />
          <path d="M12 7V4.5" />
        </svg>
      );
    case "games":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4" y="4.5" width="16" height="15" rx="2.5" />
          <path d="M9 9.5h6M9 13.5h6" />
        </svg>
      );
    case "devices":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="8" y="3.5" width="8" height="17" rx="2.2" />
          <circle cx="12" cy="16.5" r="0.9" />
        </svg>
      );
    case "settings":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="12" cy="12" r="3.2" />
          <path d="M4.8 12h2.1M17.1 12h2.1M12 4.8v2.1M12 17.1v2.1M6.6 6.6l1.5 1.5M15.9 15.9l1.5 1.5M17.4 6.6l-1.5 1.5M8.1 15.9l-1.5 1.5" />
        </svg>
      );
    case "guide":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 5.5a3 3 0 0 1 3-3h10v16h-10a3 3 0 0 0-3 3z" />
          <path d="M8 6h7M8 9h7M8 12h5" />
        </svg>
      );
    case "download":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 4.5v9.2" />
          <path d="m8.8 10.7 3.2 3.2 3.2-3.2" />
          <rect x="5" y="16.5" width="14" height="3.5" rx="1" />
        </svg>
      );
    case "logout":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M10 5.5H6a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h4" />
          <path d="M13 8.5 18 12l-5 3.5" />
          <path d="M18 12H9" />
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M9 7.5h8M9 12h8M9 16.5h8" />
          <path d="M5 7.5h0M5 12h0M5 16.5h0" />
        </svg>
      );
  }
}
