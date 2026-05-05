import type { SaveSummary } from "../../../services/types";
import { formatBytes } from "../../../utils/format";
import { formatCompactDate } from "./helpers";
import type { SaveSelectorState } from "./types";

type SaveVersionSelectorDialogProps = {
  state: SaveSelectorState;
  loading: boolean;
  error: string | null;
  selectingVersionID: string | null;
  onClose: () => void;
  onSelect: (version: SaveSummary) => void;
};

export function SaveVersionSelectorDialog({
  state,
  loading,
  error,
  selectingVersionID,
  onClose,
  onSelect
}: SaveVersionSelectorDialogProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="treegrid-modal" role="dialog" aria-modal="true" aria-labelledby="treegrid-sync-save-title" onClick={(event) => event.stopPropagation()}>
        <header className="treegrid-modal__header">
          <div>
            <h2 id="treegrid-sync-save-title">Select Sync Save</h2>
            <p>{state.displayTitle}</p>
          </div>
          <button className="treegrid-modal__close" type="button" onClick={onClose} aria-label="Close save selector">
            Close
          </button>
        </header>

        <div className="treegrid-modal__body">
          {error ? <p className="error-state">{error}</p> : null}
          {loading ? <p className="treegrid-modal__status">Loading save history...</p> : null}
          {!loading && state.versions.length === 0 ? <p className="treegrid-modal__status">No save history found.</p> : null}

          {!loading && state.versions.length > 0 ? (
            <table className="treegrid-modal-table">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Date</th>
                  <th>Size</th>
                  <th>Status</th>
                  <th>Select</th>
                </tr>
              </thead>
              <tbody>
                {state.versions.map((version, index) => {
                  const isCurrent = version.id === state.row.primarySaveID || (state.row.primarySaveID === "" && index === 0);
                  const isBusy = selectingVersionID === version.id;
                  return (
                    <tr key={version.id}>
                      <td>v{version.version}</td>
                      <td>{formatCompactDate(version.createdAt)}</td>
                      <td>{formatBytes(version.fileSize)}</td>
                      <td>{isCurrent ? <span className="treegrid-current-pill">Current Sync Save</span> : <span>Available</span>}</td>
                      <td>
                        <button className="treegrid-select-button" type="button" disabled={isCurrent || isBusy} onClick={() => onSelect(version)} aria-label={`Select version ${version.version} for sync`}>
                          {isCurrent ? "Current" : isBusy ? "Selecting..." : "Select"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : null}
        </div>
      </section>
    </div>
  );
}
