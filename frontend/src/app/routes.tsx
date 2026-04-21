import { Navigate, createBrowserRouter } from "react-router-dom";
import { AppLayout } from "./layouts/AppLayout";
import { PublicLayout } from "./layouts/PublicLayout";
import { AboutPage } from "../pages/public/AboutPage";
import { DeviceVerifyPage } from "../pages/public/DeviceVerifyPage";
import { DownloadPage } from "../pages/public/DownloadPage";
import { ForgotPasswordPage } from "../pages/public/ForgotPasswordPage";
import { GettingStartedPage } from "../pages/public/GettingStartedPage";
import { LandingPage } from "../pages/public/LandingPage";
import { LoginPage } from "../pages/public/LoginPage";
import { PrivacyPage } from "../pages/public/PrivacyPage";
import { ResetPasswordPage } from "../pages/public/ResetPasswordPage";
import { RoadmapPage } from "../pages/public/RoadmapPage";
import { SignupPage } from "../pages/public/SignupPage";
import { VerifyEmailPage } from "../pages/public/VerifyEmailPage";
import { CatalogPage } from "../pages/app/CatalogPage";
import { ConflictsPage } from "../pages/app/ConflictsPage";
import { DevicesPage } from "../pages/app/DevicesPage";
import { GamesPage } from "../pages/app/GamesPage";
import { MyGamesPage } from "../pages/app/MyGamesPage";
import { ReferralsPage } from "../pages/app/ReferralsPage";
import { SaveDetailPage } from "../pages/app/SaveDetailPage";
import { SettingsPage } from "../pages/app/SettingsPage";

function NotFoundPage(): JSX.Element {
  return (
    <section className="section-card fade-in-up">
      <h2>Pagina niet gevonden</h2>
      <p>Deze route bestaat niet in RetroSaveManager.</p>
    </section>
  );
}

export const appRouter = createBrowserRouter([
  {
    element: <PublicLayout />,
    children: [
      { index: true, element: <LandingPage /> },
      { path: "login", element: <LoginPage /> },
      { path: "signup", element: <SignupPage /> },
      { path: "forgot-password", element: <ForgotPasswordPage /> },
      { path: "reset-password", element: <ResetPasswordPage /> },
      { path: "device/:code", element: <DeviceVerifyPage /> },
      { path: "verify-email", element: <VerifyEmailPage /> },
      { path: "download", element: <DownloadPage /> },
      { path: "getting-started", element: <GettingStartedPage /> },
      { path: "roadmap", element: <RoadmapPage /> },
      { path: "about", element: <AboutPage /> },
      { path: "privacy", element: <PrivacyPage /> }
    ]
  },
  {
    path: "/app",
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/app/my-games" replace /> },
      { path: "my-games", element: <MyGamesPage /> },
      { path: "games", element: <GamesPage /> },
      { path: "saves/:saveId", element: <SaveDetailPage /> },
      { path: "catalog", element: <CatalogPage /> },
      { path: "conflicts", element: <ConflictsPage /> },
      { path: "devices", element: <DevicesPage /> },
      { path: "settings", element: <SettingsPage /> },
      { path: "referrals", element: <ReferralsPage /> },
      { path: "roadmap", element: <RoadmapPage /> },
      { path: "download", element: <DownloadPage /> },
      { path: "getting-started", element: <GettingStartedPage /> }
    ]
  },
  {
    path: "*",
    element: <NotFoundPage />
  }
]);
