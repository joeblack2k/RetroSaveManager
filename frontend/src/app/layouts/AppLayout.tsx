import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { clearFrontendAuthSession, isFrontendAuthRequired } from "../../services/authSession";

const appNav: Array<{ label: string; to: string }> = [
  { label: "My Saves", to: "/app/my-games" },
  { label: "Devices", to: "/app/devices" },
  { label: "Settings", to: "/app/settings" },
  { label: "Getting Started", to: "/app/getting-started" },
  { label: "Download", to: "/app/download" }
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
        <Link to="/" className="sidebar-brand" aria-label="RetroSaveManager home" title="RetroSaveManager">
          <span className="sidebar-brand__heading">Storage</span>
        </Link>

        <nav className="side-nav" aria-label="App">
          {appNav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              title={item.label}
              aria-label={item.label}
              className={({ isActive }) => (isActive ? "side-nav__link side-nav__link--active" : "side-nav__link")}
            >
              <span className="side-nav__label">{item.label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="side-nav__spacer" />

        {authRequired ? (
          <footer className="sidebar-user">
            <button className="sidebar-logout" type="button" aria-label="Log out" title="Log out" onClick={handleLogout}>
              Log out
            </button>
          </footer>
        ) : null}
      </aside>

      <main className="app-main">
        <Outlet />
      </main>
    </div>
  );
}
