import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";

export function LandingPage(): JSX.Element {
  return (
    <div className="grid two-cols">
      <SectionCard title="Self-hosted save sync service" subtitle="Volledig intern, geen upstream tracking, helper-friendly API surface.">
        <p>
          RetroSaveManager synchroniseert save files tussen MiSTer, RetroArch, OnionOS en desktop clients. De backend biedt root en
          <code> /v1 </code>
          alias routes voor maximale helper-compatibiliteit.
        </p>
        <div className="inline-actions">
          <Link to="/app/my-games" className="btn btn-primary">
            Open dashboard
          </Link>
          <Link to="/getting-started" className="btn btn-ghost">
            Getting started
          </Link>
        </div>
      </SectionCard>
      <SectionCard title="In scope" subtitle="User web + helper compat, zonder admin/billing/forge/manager.">
        <ul className="plain-list">
          <li>Login, signup, device verification, wachtwoordflows</li>
          <li>Games, save detail, catalog, conflicts, devices, settings</li>
          <li>Roadmap, referrals, download, privacy, about</li>
          <li>Eenduidige save-root structuur voor backup/restore</li>
        </ul>
      </SectionCard>
    </div>
  );
}
