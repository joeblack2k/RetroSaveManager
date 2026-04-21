import { SectionCard } from "../../components/SectionCard";

export function AboutPage(): JSX.Element {
  return (
    <SectionCard title="About RetroSaveManager" subtitle="Self-hosted save sync compatible with existing helper ecosystems.">
      <p>
        Dit project is gebouwd voor interne trusted omgevingen met een no-auth compatibiliteitsmodus. De focus ligt op helper
        protocol-compatibiliteit, voorspelbare data-opslag en onderhoudbare code.
      </p>
      <p>
        Scope: user web flows + helper compat. Buiten scope: admin, billing, manager, forge modules.
      </p>
    </SectionCard>
  );
}
