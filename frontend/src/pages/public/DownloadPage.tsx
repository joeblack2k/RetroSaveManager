import { SectionCard } from "../../components/SectionCard";

const helperTargets = [
  "RetroArch helper",
  "MiSTer helper",
  "OnionOS helper",
  "OpenEmu helper"
];

export function DownloadPage(): JSX.Element {
  return (
    <div className="grid two-cols">
      <SectionCard title="Helper apps" subtitle="Gebruik bestaande 1Retro helper apps met een aangepaste API-URL.">
        <ul className="plain-list">
          {helperTargets.map((target) => (
            <li key={target}>{target}</li>
          ))}
        </ul>
        <p>
          Stel in de helper in:
          <code> ONE_RETRO_API_URL=http://jouw-host </code>
          en optioneel
          <code> ONE_RETRO_APP_PASSWORD </code>
          .
        </p>
      </SectionCard>
      <SectionCard title="Deploy modes" subtitle="Direct :80, optioneel TLS :443, optioneel macvlan eigen IP.">
        <p>Gebruik de compose profielen uit de deployment-map om de juiste netwerkmodus te kiezen voor jouw LAN setup.</p>
      </SectionCard>
    </div>
  );
}
