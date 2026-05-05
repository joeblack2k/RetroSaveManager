import { Link } from "react-router-dom";
import type { MemoryCardEntry, SaveSummary } from "../../../services/types";
import { formatBytes } from "../../../utils/format";
import { buildSaveDetailsHref } from "../../../utils/saveRows";
import { normalizeRegionCode, regionToFlagEmoji } from "./helpers";
import type { OpenDownloadModal } from "./types";

type PlayStationSavePickerProps = {
  entries: MemoryCardEntry[];
  saveId: string;
  latest: SaveSummary | null;
  openDownloadModal: OpenDownloadModal;
};

export function PlayStationSavePicker({ entries, saveId, latest, openDownloadModal }: PlayStationSavePickerProps): JSX.Element {
  return (
    <section className="save-detail-panel" aria-labelledby="save-detail-picker-title">
      <div className="save-detail-panel__header">
        <div>
          <p className="save-detail-eyebrow">Memory card entries</p>
          <h3 id="save-detail-picker-title">Select a save</h3>
          <p>Each entry has its own details, version history, and download options.</p>
        </div>
      </div>
      <div className="save-detail-table-wrap">
        <table className="save-detail-table">
          <thead>
            <tr>
              <th>Save</th>
              <th>Region</th>
              <th>Size</th>
              <th>Details</th>
              <th>Download</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, index) => {
              const entryLogicalKey = (entry.logicalKey || "").trim();
              const canOpen = entryLogicalKey !== "";
              return (
                <tr key={`${entryLogicalKey || entry.productCode || entry.directoryName || entry.title}-${index}`}>
                  <td>
                    <div className="save-detail-entry-cell">
                      {entry.iconDataUrl ? (
                        <img className="memory-card-entry-preview" src={entry.iconDataUrl} alt={`${entry.title} icon`} loading="lazy" />
                      ) : (
                        <div className="memory-card-entry-preview memory-card-entry-preview--empty" aria-hidden="true" />
                      )}
                      <div>
                        <strong>{entry.title}</strong>
                        <span>{entry.productCode || entry.directoryName || "Memory card save"}</span>
                      </div>
                    </div>
                  </td>
                  <td>{regionToFlagEmoji((entry.regionCode || "UNKNOWN").toString())} {normalizeRegionCode((entry.regionCode || "UNKNOWN").toString())}</td>
                  <td>{formatBytes(entry.totalSizeBytes || entry.sizeBytes || 0)}</td>
                  <td>
                    {canOpen ? (
                      <Link className="saves-action-link" to={buildSaveDetailsHref({ primarySaveID: saveId, psLogicalKey: entryLogicalKey })}>
                        Details
                      </Link>
                    ) : (
                      <span>-</span>
                    )}
                  </td>
                  <td>
                    {canOpen ? (
                      <button className="saves-action-btn" type="button" onClick={() => openDownloadModal(entry.title, { saveId, psLogicalKey: entryLogicalKey }, latest?.downloadProfiles)}>
                        Download
                      </button>
                    ) : (
                      <span>-</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
