import type { SaveSummary } from "../../../services/types";
import type { SaveInsightModel } from "../../../utils/saveInsights";
import type { DetailMetric } from "./types";

type TechnicalDetailsPanelProps = {
  latest: SaveSummary | null;
  insight: SaveInsightModel | null;
  languageCodes: string[];
};

export function TechnicalDetailsPanel({ latest, insight, languageCodes }: TechnicalDetailsPanelProps): JSX.Element | null {
  if (!latest && !insight) {
    return null;
  }
  const technicalRows = insight?.rows.filter((row) => row.kind !== "gameplay") ?? [];
  const rows: DetailMetric[] = [
    { label: "Filename", value: latest?.filename || "-" },
    { label: "Format", value: latest?.format || "-" },
    { label: "SHA256", value: latest?.sha256 || "-" },
    { label: "Languages", value: languageCodes.length > 0 ? languageCodes.join(", ") : "-" },
    { label: "Source profile", value: latest?.sourceArtifactProfile || "-" },
    { label: "Runtime profile", value: latest?.runtimeProfile || "-" }
  ];
  for (const row of technicalRows) {
    rows.push({ label: row.label, value: row.value });
  }

  return (
    <details className="save-detail-technical">
      <summary>Verified technical data</summary>
      <div className="save-detail-technical-grid">
        {rows.filter((row) => row.value && row.value !== "-").map((row) => (
          <div className="save-detail-tech-row" key={`${row.label}:${row.value}`}>
            <span>{row.label}</span>
            <strong>{row.value}</strong>
          </div>
        ))}
      </div>
      {insight?.evidence.length ? (
        <div className="save-detail-evidence">
          <span>Parser evidence</span>
          <ul>
            {insight.evidence.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </details>
  );
}
