import type { DeviceConfigSource } from "../../../services/types";

export function DevicePolicyPreview({ syncAll, sources }: { syncAll: boolean; sources: DeviceConfigSource[] }): JSX.Element {
  const systems = Array.from(new Set(sources.flatMap((source) => source.systems ?? []))).sort();
  const managed = sources.filter((source) => source.managed).length;
  return (
    <aside className="device-policy-preview" aria-label="Policy preview">
      <div>
        <span>Policy preview</span>
        <strong>{syncAll ? "Sync all allowed systems" : `${systems.length} manually selected`}</strong>
      </div>
      <p>
        {sources.length} source profiles, {managed} backend-managed. {systems.length > 0 ? `Consoles: ${systems.join(", ")}.` : "No backend console sources yet."}
      </p>
    </aside>
  );
}
