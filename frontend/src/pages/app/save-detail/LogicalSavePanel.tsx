import type { MemoryCardEntry, SaveSummary } from "../../../services/types";
import { formatBytes } from "../../../utils/format";

type LogicalSavePanelProps = {
  entry: MemoryCardEntry;
  latest: SaveSummary | null;
};

export function LogicalSavePanel({ entry, latest }: LogicalSavePanelProps): JSX.Element {
  return (
    <section className="save-detail-panel save-detail-logical" aria-label="Logical save">
      {entry.iconDataUrl ? <img className="memory-card-entry-preview" src={entry.iconDataUrl} alt={`${entry.title} icon`} loading="lazy" /> : null}
      <div>
        <p className="save-detail-eyebrow">Logical save</p>
        <h3>{entry.title}</h3>
        <p>
          {entry.productCode ? `${entry.productCode} / ` : ""}
          {entry.directoryName || "PlayStation entry"}
        </p>
      </div>
      <div className="save-detail-logical__stats">
        {entry.slot > 0 ? <span>Slot {entry.slot}</span> : null}
        {entry.blocks > 0 ? <span>{entry.blocks} blocks</span> : null}
        <span>{formatBytes(entry.sizeBytes || latest?.fileSize || 0)}</span>
      </div>
    </section>
  );
}
