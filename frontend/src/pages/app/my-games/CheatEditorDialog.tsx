import type { SaveCheatEditorState, SaveCheatField } from "../../../services/types";
import type { SaveRow } from "../../../utils/saveRows";
import { defaultCheatSlotId } from "./helpers";

type CheatEditorDialogProps = {
  row: SaveRow;
  displayTitle: string;
  data: SaveCheatEditorState | null;
  loading: boolean;
  error: string | null;
  applying: boolean;
  selectedSlot: string;
  pendingUpdates: Record<string, unknown>;
  currentValues: Record<string, unknown>;
  onClose: () => void;
  onSlotChange: (slotId: string) => void;
  onResetDraft: () => void;
  onApplyPreset: (updates: Record<string, unknown> | undefined) => void;
  onFieldChange: (field: SaveCheatField, value: unknown) => void;
  onApply: () => void;
};

export function CheatEditorDialog({
  row,
  displayTitle,
  data,
  loading,
  error,
  applying,
  selectedSlot,
  pendingUpdates,
  currentValues,
  onClose,
  onSlotChange,
  onResetDraft,
  onApplyPreset,
  onFieldChange,
  onApply
}: CheatEditorDialogProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="treegrid-modal treegrid-modal--wide" role="dialog" aria-modal="true" aria-labelledby="treegrid-cheat-title" onClick={(event) => event.stopPropagation()}>
        <header className="treegrid-modal__header">
          <div>
            <h2 id="treegrid-cheat-title">Cheat Editor</h2>
            <p>{displayTitle || row.gameName}</p>
          </div>
          <button className="treegrid-modal__close" type="button" onClick={onClose} aria-label="Close cheat editor">
            Close
          </button>
        </header>

        <div className="treegrid-modal__body treegrid-cheat-body">
          {error ? <p className="error-state">{error}</p> : null}
          {loading ? <p className="treegrid-modal__status">Loading cheat options...</p> : null}
          {!loading && data && !data.supported ? <p className="treegrid-modal__status">No safe cheat editor is available for this save.</p> : null}

          {!loading && data?.supported ? (
            <>
              {data.selector?.options && data.selector.options.length > 0 ? (
                <label className="treegrid-cheat-slot-picker">
                  <span>{data.selector.label}</span>
                  <select value={selectedSlot || defaultCheatSlotId(data)} onChange={(event) => onSlotChange(event.target.value)}>
                    {data.selector.options.map((option) => (
                      <option key={option.id} value={option.id}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}

              {data.presets && data.presets.length > 0 ? (
                <div className="treegrid-cheat-presets">
                  <p className="treegrid-cheat-presets__label">Presets</p>
                  <div className="treegrid-cheat-presets__actions">
                    {data.presets.map((preset) => (
                      <button key={preset.id} className="treegrid-cheat-preset" type="button" onClick={() => onApplyPreset(preset.updates)} title={preset.description || preset.label}>
                        {preset.label}
                      </button>
                    ))}
                    {Object.keys(pendingUpdates).length > 0 ? (
                      <button className="treegrid-cheat-preset treegrid-cheat-preset--ghost" type="button" onClick={onResetDraft}>
                        Reset Draft
                      </button>
                    ) : null}
                  </div>
                </div>
              ) : null}

              <div className="treegrid-cheat-sections">
                {data.sections?.map((section) => (
                  <section key={section.id} className="treegrid-cheat-section">
                    <header className="treegrid-cheat-section__header">
                      <h3>{section.title}</h3>
                    </header>
                    <div className="treegrid-cheat-fields">
                      {section.fields.map((field) => {
                        const currentValue = pendingUpdates[field.id] ?? currentValues[field.id];
                        return (
                          <div key={field.id} className="treegrid-cheat-field">
                            {renderCheatField(field, currentValue, onFieldChange)}
                          </div>
                        );
                      })}
                    </div>
                  </section>
                ))}
              </div>

              <footer className="treegrid-cheat-actions">
                <button className="treegrid-select-button" type="button" onClick={onApply} disabled={applying}>
                  {applying ? "Applying..." : "Apply Cheats"}
                </button>
              </footer>
            </>
          ) : null}
        </div>
      </section>
    </div>
  );
}

function renderCheatField(
  field: SaveCheatField,
  currentValue: unknown,
  onChange: (field: SaveCheatField, value: unknown) => void
): JSX.Element {
  switch (field.type) {
    case "boolean": {
      const checked = Boolean(currentValue);
      return (
        <label className="treegrid-cheat-toggle">
          <input type="checkbox" checked={checked} onChange={(event) => onChange(field, event.target.checked)} />
          <span>{field.label}</span>
        </label>
      );
    }
    case "integer": {
      const value = typeof currentValue === "number" ? currentValue : 0;
      return (
        <label className="treegrid-cheat-input">
          <span>{field.label}</span>
          <input type="number" min={field.min} max={field.max} step={field.step ?? 1} value={value} onChange={(event) => onChange(field, Number(event.target.value || 0))} />
        </label>
      );
    }
    case "enum": {
      const value = typeof currentValue === "string" ? currentValue : field.options?.[0]?.id ?? "";
      return (
        <label className="treegrid-cheat-input">
          <span>{field.label}</span>
          <select value={value} onChange={(event) => onChange(field, event.target.value)}>
            {(field.options ?? []).map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      );
    }
    case "bitmask": {
      const selected = Array.isArray(currentValue) ? currentValue.filter((item): item is string => typeof item === "string") : [];
      return (
        <fieldset className="treegrid-cheat-bitmask">
          <legend>{field.label}</legend>
          <div className="treegrid-cheat-bitmask__options">
            {(field.bits ?? []).map((bit) => {
              const checked = selected.includes(bit.id);
              return (
                <label key={bit.id} className="treegrid-cheat-toggle treegrid-cheat-toggle--compact">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={(event) => {
                      const next = event.target.checked ? [...selected, bit.id] : selected.filter((item) => item !== bit.id);
                      onChange(field, next);
                    }}
                  />
                  <span>{bit.label}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
      );
    }
    default:
      return (
        <div className="treegrid-cheat-unsupported">
          <strong>{field.label}</strong>
          <span>Unsupported field type: {field.type}</span>
        </div>
      );
  }
}
