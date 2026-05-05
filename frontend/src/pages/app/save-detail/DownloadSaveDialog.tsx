import { buildSaveDownloadHref } from "../../../utils/saveRows";
import type { DownloadModalState } from "./types";

type DownloadSaveDialogProps = {
  state: DownloadModalState;
  onClose: () => void;
};

export function DownloadSaveDialog({ state, onClose }: DownloadSaveDialogProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="treegrid-modal" role="dialog" aria-modal="true" aria-labelledby="save-detail-download-title" onClick={(event) => event.stopPropagation()}>
        <header className="treegrid-modal__header">
          <div>
            <h2 id="save-detail-download-title">Download Save</h2>
            <p>{state.title}</p>
          </div>
          <button className="treegrid-modal__close" type="button" onClick={onClose} aria-label="Close download options">
            Close
          </button>
        </header>

        <div className="treegrid-modal__body">
          <table className="treegrid-modal-table">
            <thead>
              <tr>
                <th>Profile</th>
                <th>Extension</th>
                <th>Notes</th>
                <th>Download</th>
              </tr>
            </thead>
            <tbody>
              {state.profiles.map((profile) => (
                <tr key={profile.id}>
                  <td>{profile.label}</td>
                  <td>{profile.targetExtension || "-"}</td>
                  <td>{profile.note || "-"}</td>
                  <td>
                    <a className="saves-action-link" href={buildSaveDownloadHref(state.request, profile.id !== "original" ? profile.id : undefined)}>
                      Download
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
