import { useEffect, useMemo, useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { clearFrontendAuthSession, isFrontendAuthRequired } from "../../services/authSession";
import { enableAutoAppPasswordEnrollment, getAutoAppPasswordEnrollmentStatus } from "../../services/retrosaveApi";

const appNav: Array<{ label: string; to: string }> = [
  { label: "My Saves", to: "/app/my-games" },
  { label: "Devices", to: "/app/devices" },
  { label: "Settings", to: "/app/settings" }
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

        <SidebarHelperPanel />

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

function SidebarHelperPanel(): JSX.Element {
  const [enabledUntil, setEnabledUntil] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [loading, setLoading] = useState(true);
  const [activating, setActivating] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadStatus(): Promise<void> {
      try {
        const status = await getAutoAppPasswordEnrollmentStatus();
        if (!cancelled) {
          setEnabledUntil(status.active ? status.enabledUntil ?? null : null);
        }
      } catch {
        if (!cancelled) {
          setEnabledUntil(null);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void loadStatus();

    return () => {
      cancelled = true;
    };
  }, []);

  const remainingSeconds = useMemo(() => {
    if (!enabledUntil) {
      return 0;
    }
    const target = Date.parse(enabledUntil);
    if (Number.isNaN(target)) {
      return 0;
    }
    return Math.max(0, Math.ceil((target - now) / 1000));
  }, [enabledUntil, now]);

  useEffect(() => {
    if (!enabledUntil || remainingSeconds <= 0) {
      if (enabledUntil && remainingSeconds <= 0) {
        setEnabledUntil(null);
      }
      return;
    }

    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(timer);
    };
  }, [enabledUntil, remainingSeconds]);

  async function handleActivate(): Promise<void> {
    setActivating(true);
    try {
      const status = await enableAutoAppPasswordEnrollment(15);
      setNow(Date.now());
      setEnabledUntil(status.active ? status.enabledUntil ?? null : null);
    } catch {
      setEnabledUntil(null);
    } finally {
      setActivating(false);
      setLoading(false);
    }
  }

  return (
    <section className="sidebar-helper" aria-label="Helper pairing">
      <p className="sidebar-helper__eyebrow">Helper pairing</p>
      <p className="sidebar-helper__copy">Open a 15 minute helper window.</p>
      <div className="sidebar-helper__control">
        {enabledUntil && remainingSeconds > 0 ? (
          <div className="sidebar-helper__timer" role="status" aria-live="polite">
            {formatSidebarHelperCountdown(remainingSeconds)}
          </div>
        ) : (
          <button
            className="sidebar-helper__button"
            type="button"
            onClick={() => void handleActivate()}
            disabled={loading || activating}
          >
            {activating ? "Opening..." : "Add helper"}
          </button>
        )}
      </div>
    </section>
  );
}

function formatSidebarHelperCountdown(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
