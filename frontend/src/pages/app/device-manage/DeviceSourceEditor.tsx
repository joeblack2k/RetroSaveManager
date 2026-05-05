import type { SaveSystem, DeviceConfigSource } from "../../../services/types";
import { DevicePolicyPreview } from "./DevicePolicyPreview";
import { SOURCE_KIND_OPTIONS, formatSourcePaths } from "./options";

type DeviceSourceEditorProps = {
  systems: SaveSystem[];
  sources: DeviceConfigSource[];
  sourcesDirty: boolean;
  syncAll: boolean;
  draftSystemSlug: string;
  draftKind: string;
  draftProfile: string;
  draftProfileOptions: Array<{ value: string; label: string }>;
  draftLabel: string;
  draftSavePath: string;
  draftRomPath: string;
  onSystemChange: (value: string) => void;
  onKindChange: (value: string) => void;
  onProfileChange: (value: string) => void;
  onLabelChange: (value: string) => void;
  onSavePathChange: (value: string) => void;
  onRomPathChange: (value: string) => void;
  onAddSource: () => void;
  onRemoveSource: (id: string) => void;
};

export function DeviceSourceEditor({
  systems,
  sources,
  sourcesDirty,
  syncAll,
  draftSystemSlug,
  draftKind,
  draftProfile,
  draftProfileOptions,
  draftLabel,
  draftSavePath,
  draftRomPath,
  onSystemChange,
  onKindChange,
  onProfileChange,
  onLabelChange,
  onSavePathChange,
  onRomPathChange,
  onAddSource,
  onRemoveSource
}: DeviceSourceEditorProps): JSX.Element {
  return (
    <section className="device-source-editor device-manage-panel">
      <div className="device-source-editor__header">
        <div>
          <h3>Add console source</h3>
          <p>Add folders the always-on helper should include in its next backend-managed config policy.</p>
        </div>
        {sourcesDirty ? <span>Unsaved changes</span> : null}
      </div>

      <div className="device-source-form">
        <label className="field">
          <span>Console</span>
          <select value={draftSystemSlug} onChange={(event) => onSystemChange(event.target.value)}>
            {systems
              .filter((system) => Boolean(system.slug))
              .map((system) => (
                <option key={system.slug} value={system.slug}>
                  {system.name}
                </option>
              ))}
          </select>
        </label>
        <label className="field">
          <span>Runtime kind</span>
          <select value={draftKind} onChange={(event) => onKindChange(event.target.value)}>
            {SOURCE_KIND_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Profile</span>
          <select value={draftProfile} onChange={(event) => onProfileChange(event.target.value)}>
            {draftProfileOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Label</span>
          <input value={draftLabel} onChange={(event) => onLabelChange(event.target.value)} placeholder="Optional display name" />
        </label>
        <label className="field device-source-form__wide">
          <span>Save folder</span>
          <input value={draftSavePath} onChange={(event) => onSavePathChange(event.target.value)} placeholder="/media/snes9x/saves" />
        </label>
        <label className="field device-source-form__wide">
          <span>ROM folder</span>
          <input value={draftRomPath} onChange={(event) => onRomPathChange(event.target.value)} placeholder="/media/snes9x/roms (optional)" />
        </label>
      </div>
      <button className="btn btn-ghost" type="button" onClick={onAddSource}>
        Add console
      </button>

      <div className="device-source-list">
        {sources.length === 0 ? <p>No helper config sources reported yet.</p> : null}
        {sources.map((source) => (
          <article key={source.id} className="device-source-row">
            <div>
              <strong>{source.label || source.id}</strong>
              <p>{[source.kind, source.profile, source.origin].filter(Boolean).join(" / ") || "Unknown source"}</p>
              <small>
                {(source.systems ?? []).join(", ") || "No systems"} · {formatSourcePaths(source.savePaths ?? (source.savePath ? [source.savePath] : []))}
              </small>
            </div>
            <button className="btn btn-ghost btn-danger" type="button" onClick={() => onRemoveSource(source.id)}>
              Remove
            </button>
          </article>
        ))}
      </div>
      <DevicePolicyPreview syncAll={syncAll} sources={sources} />
    </section>
  );
}
