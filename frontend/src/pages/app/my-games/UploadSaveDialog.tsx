import type { FormEvent } from "react";
import type { SaveUploadPreviewItem } from "../../../services/types";
import { formatBytes } from "../../../utils/format";
import { runtimeProfileOptionsForSystem } from "./helpers";

type UploadSaveDialogProps = {
  uploadSystem: string;
  uploadSlotName: string;
  uploadRomSha1: string;
  uploadWiiTitleId: string;
  uploadRuntimeProfile: string;
  uploadPreviewItems: SaveUploadPreviewItem[] | null;
  uploadPreviewBusy: boolean;
  uploadBusy: boolean;
  uploadError: string | null;
  uploadResult: string | null;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onPreview: () => void;
  onFileChange: (file: File | null) => void;
  onSystemChange: (value: string) => void;
  onRuntimeProfileChange: (value: string) => void;
  onSlotNameChange: (value: string) => void;
  onRomSha1Change: (value: string) => void;
  onWiiTitleIdChange: (value: string) => void;
};

export function UploadSaveDialog({
  uploadSystem,
  uploadSlotName,
  uploadRomSha1,
  uploadWiiTitleId,
  uploadRuntimeProfile,
  uploadPreviewItems,
  uploadPreviewBusy,
  uploadBusy,
  uploadError,
  uploadResult,
  onClose,
  onSubmit,
  onPreview,
  onFileChange,
  onSystemChange,
  onRuntimeProfileChange,
  onSlotNameChange,
  onRomSha1Change,
  onWiiTitleIdChange
}: UploadSaveDialogProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="treegrid-modal" role="dialog" aria-modal="true" aria-labelledby="treegrid-upload-title" onClick={(event) => event.stopPropagation()}>
        <header className="treegrid-modal__header">
          <div>
            <h2 id="treegrid-upload-title">Upload Save</h2>
            <p>Preview first, then import only validated saves. Rejected files are quarantined for review.</p>
          </div>
          <button className="treegrid-modal__close" type="button" onClick={onClose} aria-label="Close upload">
            Close
          </button>
        </header>

        <form className="treegrid-upload-form" onSubmit={onSubmit}>
          <div className="treegrid-upload-grid">
            <label className="treegrid-upload-field treegrid-upload-field--wide">
              <span>Save file or zip</span>
              <input
                type="file"
                accept=".zip,.bin,.sav,.srm,.sa1,.ram,.rtc,.gme,.eep,.sra,.fla,.mpk,.cpk,.mcr,.mcd,.mc,.ps2,.vms,.dci,.bkr,.bcr,.bup,.brm,.eeprom,.nvram"
                onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
              />
            </label>
          </div>

          <details className="treegrid-upload-advanced">
            <summary>Advanced metadata</summary>
            <div className="treegrid-upload-grid">
              <label className="treegrid-upload-field">
                <span>System</span>
                <select value={uploadSystem} onChange={(event) => onSystemChange(event.target.value)}>
                  <option value="">Auto-detect when possible</option>
                  <option value="wii">Nintendo Wii</option>
                  <option value="n64">Nintendo 64</option>
                  <option value="snes">Super Nintendo</option>
                  <option value="nes">Nintendo Entertainment System</option>
                  <option value="gba">Game Boy Advance</option>
                  <option value="gameboy">Game Boy</option>
                  <option value="pc-engine">PC Engine / TurboGrafx-16</option>
                  <option value="atari-lynx">Atari Lynx</option>
                  <option value="wonderswan">WonderSwan</option>
                  <option value="genesis">Genesis / Mega Drive</option>
                  <option value="master-system">Master System</option>
                  <option value="game-gear">Game Gear</option>
                  <option value="sega-cd">Sega CD / Mega-CD</option>
                  <option value="sega-32x">Sega 32X</option>
                  <option value="sg-1000">SG-1000</option>
                  <option value="dreamcast">Dreamcast</option>
                  <option value="saturn">Saturn</option>
                  <option value="neogeo">Neo Geo</option>
                  <option value="colecovision">ColecoVision</option>
                  <option value="atari-jaguar">Atari Jaguar</option>
                  <option value="3do">3DO</option>
                  <option value="psx">PlayStation</option>
                  <option value="ps2">PlayStation 2</option>
                </select>
              </label>
              <label className="treegrid-upload-field">
                <span>Runtime profile</span>
                <select value={uploadRuntimeProfile} onChange={(event) => onRuntimeProfileChange(event.target.value)}>
                  <option value="">Original / backend default</option>
                  {runtimeProfileOptionsForSystem(uploadSystem).map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="treegrid-upload-field">
                <span>Slot name</span>
                <input type="text" value={uploadSlotName} onChange={(event) => onSlotNameChange(event.target.value)} placeholder="default" />
              </label>
              <label className="treegrid-upload-field">
                <span>ROM SHA1</span>
                <input type="text" value={uploadRomSha1} onChange={(event) => onRomSha1Change(event.target.value)} placeholder="optional but recommended" />
              </label>
              <label className="treegrid-upload-field">
                <span>Wii title code</span>
                <input type="text" value={uploadWiiTitleId} onChange={(event) => onWiiTitleIdChange(event.target.value.toUpperCase())} placeholder="SB4P" maxLength={4} />
              </label>
            </div>
          </details>

          <p className="treegrid-upload-hint">
            Wii zip uploads can contain <code>private/wii/title/SB4P/data.bin</code> or <code>SB4P/data.bin</code>. Raw Wii <code>data.bin</code> uploads should include the title code.
          </p>
          {uploadError ? <p className="error-state">{uploadError}</p> : null}
          {uploadResult ? <p className="treegrid-modal__status">{uploadResult}</p> : null}
          {uploadPreviewItems ? <UploadPreviewTable items={uploadPreviewItems} /> : null}

          <footer className="treegrid-upload-actions">
            <button className="treegrid-select-button treegrid-select-button--ghost" type="button" disabled={uploadPreviewBusy || uploadBusy} onClick={onPreview}>
              {uploadPreviewBusy ? "Previewing..." : "Preview"}
            </button>
            <button className="treegrid-select-button" type="submit" disabled={uploadBusy || uploadPreviewBusy || !uploadPreviewItems || uploadPreviewItems.every((item) => !item.accepted)}>
              {uploadBusy ? "Importing..." : "Import Accepted"}
            </button>
          </footer>
        </form>
      </section>
    </div>
  );
}

function UploadPreviewTable({ items }: { items: SaveUploadPreviewItem[] }): JSX.Element {
  return (
    <div className="treegrid-upload-preview" aria-label="Upload validation preview">
      <table className="treegrid-modal-table">
        <thead>
          <tr>
            <th>Status</th>
            <th>Save</th>
            <th>System</th>
            <th>Validation</th>
            <th>Size</th>
            <th>Reason</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={`${item.sourcePath || item.filename}:${item.sha256}`} className={item.accepted ? "treegrid-preview-row--accepted" : "treegrid-preview-row--rejected"}>
              <td>
                <span className={item.accepted ? "treegrid-preview-pill treegrid-preview-pill--ok" : "treegrid-preview-pill treegrid-preview-pill--bad"}>
                  {item.accepted ? "Accepted" : "Rejected"}
                </span>
              </td>
              <td>
                <strong>{item.displayTitle || item.filename}</strong>
                <small>{item.sourcePath || item.filename}</small>
              </td>
              <td>{item.systemName || item.systemSlug || "Unknown"}</td>
              <td>{item.trustLevel || item.parserLevel || "media-verified"}</td>
              <td>{formatBytes(item.sizeBytes)}</td>
              <td>{item.reason || item.warnings?.[0] || "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
