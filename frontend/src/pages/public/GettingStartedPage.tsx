import { SectionCard } from "../../components/SectionCard";

export function GettingStartedPage(): JSX.Element {
  return (
    <SectionCard title="Getting started" subtitle="Snelle setup voor LAN-only self-hosting.">
      <ol className="plain-list ordered">
        <li>Zet in deploy `.env` je hostnaam, save-root en optionele macvlan settings.</li>
        <li>Start de stack met `./scripts/up.sh direct` in de `deploy` map.</li>
        <li>Wijs in AdGuard DNS een interne hostnaam naar je docker host IP.</li>
        <li>Configureer helper apps met dezelfde interne hostnaam als API endpoint.</li>
      </ol>
    </SectionCard>
  );
}
