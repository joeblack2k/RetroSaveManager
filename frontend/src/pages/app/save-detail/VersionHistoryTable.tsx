import type { SaveSummary } from "../../../services/types";
import { formatBytes, formatDate } from "../../../utils/format";
import type { OpenDownloadModal } from "./types";

type VersionHistoryTableProps = {
  versions: SaveSummary[];
  displayTitle: string;
  effectiveLogicalKey: string;
  saveId: string;
  rollbackingId: string | null;
  requestRollback: (target: SaveSummary) => void;
  openDownloadModal: OpenDownloadModal;
};

export function VersionHistoryTable({
  versions,
  displayTitle,
  effectiveLogicalKey,
  saveId,
  rollbackingId,
  requestRollback,
  openDownloadModal
}: VersionHistoryTableProps): JSX.Element {
  return (
    <section className="save-detail-panel" aria-labelledby="save-detail-history-title">
      <div className="save-detail-panel__header">
        <div>
          <p className="save-detail-eyebrow">History</p>
          <h3 id="save-detail-history-title">Sync versions</h3>
          <p>Only meaningful controls are shown here. Full hashes and file internals live under technical data.</p>
        </div>
      </div>
      <div className="save-detail-table-wrap">
        <table className="save-detail-table">
          <thead>
            <tr>
              <th>Version</th>
              <th>Date</th>
              <th>Size</th>
              <th>Status</th>
              <th>Download</th>
              <th>Rollback</th>
            </tr>
          </thead>
          <tbody>
            {versions.map((version, index) => {
              const isLatest = index === 0;
              const isBusy = rollbackingId === version.id;
              return (
                <tr key={version.id}>
                  <td>v{version.version}</td>
                  <td>{formatDate(version.createdAt)}</td>
                  <td>{formatBytes(version.fileSize)}</td>
                  <td>{isLatest ? <span className="save-detail-status">Current sync</span> : <span className="save-detail-status save-detail-status--old">History</span>}</td>
                  <td>
                    <button
                      className="saves-action-btn"
                      type="button"
                      onClick={() =>
                        openDownloadModal(displayTitle, {
                          saveId: effectiveLogicalKey ? saveId : version.id,
                          psLogicalKey: effectiveLogicalKey || undefined,
                          revisionId: effectiveLogicalKey ? version.id : undefined
                        }, version.downloadProfiles)
                      }
                    >
                      Download
                    </button>
                  </td>
                  <td>
                    <button className="saves-action-btn" type="button" disabled={isLatest || isBusy} onClick={() => requestRollback(version)}>
                      {isLatest ? "Current" : isBusy ? "..." : "Rollback"}
                    </button>
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
