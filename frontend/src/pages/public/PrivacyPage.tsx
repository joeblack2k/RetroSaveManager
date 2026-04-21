import { SectionCard } from "../../components/SectionCard";

export function PrivacyPage(): JSX.Element {
  return (
    <SectionCard title="Privacy" subtitle="Intern gebruik zonder externe analytics/tracking scripts.">
      <ul className="plain-list">
        <li>Geen upstream Plausible of Sentry scripts in deze frontend.</li>
        <li>Data blijft lokaal in jouw eigen Docker volumes.</li>
        <li>Voor publieke blootstelling is extra auth/reverse-proxy hardening vereist.</li>
      </ul>
    </SectionCard>
  );
}
