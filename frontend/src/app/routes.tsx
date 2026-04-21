import { Navigate, createBrowserRouter, useLocation } from "react-router-dom";
import { AppLayout } from "./layouts/AppLayout";
import { PublicLayout } from "./layouts/PublicLayout";
import { AboutPage } from "../pages/public/AboutPage";
import { DeviceVerifyPage } from "../pages/public/DeviceVerifyPage";
import { DownloadPage } from "../pages/public/DownloadPage";
import { ForgotPasswordPage } from "../pages/public/ForgotPasswordPage";
import { GettingStartedPage } from "../pages/public/GettingStartedPage";
import { LoginPage } from "../pages/public/LoginPage";
import { PrivacyPage } from "../pages/public/PrivacyPage";
import { ResetPasswordPage } from "../pages/public/ResetPasswordPage";
import { SignupPage } from "../pages/public/SignupPage";
import { VerifyEmailPage } from "../pages/public/VerifyEmailPage";
import { ConflictsPage } from "../pages/app/ConflictsPage";
import { DeviceManagePage } from "../pages/app/DeviceManagePage";
import { DevicesPage } from "../pages/app/DevicesPage";
import { GamesPage } from "../pages/app/GamesPage";
import { MyGamesPage } from "../pages/app/MyGamesPage";
import { SaveDetailPage } from "../pages/app/SaveDetailPage";
import { SettingsPage } from "../pages/app/SettingsPage";
import { hasFrontendAuthSession, isFrontendAuthRequired } from "../services/authSession";

function NotFoundPage(): JSX.Element {
  return (
    <section className="section-card fade-in-up">
      <h2>Pagina niet gevonden</h2>
      <p>Deze route bestaat niet in RetroSaveManager.</p>
    </section>
  );
}

function RootRedirect(): JSX.Element {
  if (isFrontendAuthRequired() && !hasFrontendAuthSession()) {
    return <Navigate to="/login" replace />;
  }
  return <Navigate to="/app/my-games" replace />;
}

function RequireFrontendAuth(): JSX.Element {
  const location = useLocation();

  if (!isFrontendAuthRequired()) {
    return <AppLayout />;
  }
  if (hasFrontendAuthSession()) {
    return <AppLayout />;
  }

  const next = `${location.pathname}${location.search}${location.hash}`;
  return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />;
}

function AuthEntryRoute({ children }: { children: JSX.Element }): JSX.Element {
  if (!isFrontendAuthRequired()) {
    return <Navigate to="/app/my-games" replace />;
  }
  if (hasFrontendAuthSession()) {
    return <Navigate to="/app/my-games" replace />;
  }
  return children;
}

export const appRouter = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />
  },
  {
    element: <PublicLayout />,
    children: [
      {
        path: "login",
        element: (
          <AuthEntryRoute>
            <LoginPage />
          </AuthEntryRoute>
        )
      },
      {
        path: "signup",
        element: (
          <AuthEntryRoute>
            <SignupPage />
          </AuthEntryRoute>
        )
      },
      {
        path: "forgot-password",
        element: (
          <AuthEntryRoute>
            <ForgotPasswordPage />
          </AuthEntryRoute>
        )
      },
      {
        path: "reset-password",
        element: (
          <AuthEntryRoute>
            <ResetPasswordPage />
          </AuthEntryRoute>
        )
      },
      { path: "device/:code", element: <DeviceVerifyPage /> },
      { path: "verify-email", element: <VerifyEmailPage /> },
      { path: "download", element: <DownloadPage /> },
      { path: "getting-started", element: <GettingStartedPage /> },
      { path: "about", element: <AboutPage /> },
      { path: "privacy", element: <PrivacyPage /> }
    ]
  },
  {
    path: "/app",
    element: <RequireFrontendAuth />,
    children: [
      { index: true, element: <Navigate to="/app/my-games" replace /> },
      { path: "my-games", element: <MyGamesPage /> },
      { path: "games", element: <GamesPage /> },
      { path: "saves/:saveId", element: <SaveDetailPage /> },
      { path: "conflicts", element: <ConflictsPage /> },
      { path: "devices", element: <DevicesPage /> },
      { path: "devices/:deviceId/manage", element: <DeviceManagePage /> },
      { path: "settings", element: <SettingsPage /> },
      { path: "download", element: <DownloadPage /> },
      { path: "getting-started", element: <GettingStartedPage /> }
    ]
  },
  {
    path: "*",
    element: <NotFoundPage />
  }
]);
