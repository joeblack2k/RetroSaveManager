import { FormEvent } from "react";
import type { AppPassword } from "../../../services/types";
import { formatDate } from "../../../utils/format";

type HelperKeysPanelProps = {
  appPasswords: AppPassword[];
  nameDraft: string;
  generatedKey: string | null;
  generateBusy: boolean;
  copyStatus: string | null;
  onNameChange: (value: string) => void;
  onGenerate: (event: FormEvent) => void;
  onCopy: () => void;
  onHideKey: () => void;
  onRevoke: (key: AppPassword) => void;
};

export function HelperKeysPanel({
  appPasswords,
  nameDraft,
  generatedKey,
  generateBusy,
  copyStatus,
  onNameChange,
  onGenerate,
  onCopy,
  onHideKey,
  onRevoke
}: HelperKeysPanelProps): JSX.Element {
  return (
    <details className="settings-disclosure">
      <summary>
        <span>Helper keys</span>
        <small>{appPasswords.length} saved keys</small>
      </summary>
      <div className="settings-disclosure__body">
        <p>
          Use this only for fixed helper credentials. For normal onboarding, use <strong>Add helper</strong> in the
          sidebar.
        </p>
        <form className="inline-actions" onSubmit={onGenerate}>
          <input
            value={nameDraft}
            onChange={(event) => onNameChange(event.target.value)}
            placeholder="Name, for example SteamDeck"
            aria-label="App password name"
          />
          <button className="btn btn-primary" type="submit" disabled={generateBusy}>
            {generateBusy ? "Generating..." : "Generate key"}
          </button>
        </form>

        {generatedKey ? (
          <div className="generated-key-box" role="status" aria-live="polite">
            <p>
              <strong>New key:</strong> <code>{generatedKey}</code>
            </p>
            <div className="inline-actions">
              <button className="btn btn-ghost" type="button" onClick={onCopy}>
                Copy
              </button>
              <button className="btn btn-ghost" type="button" onClick={onHideKey}>
                Hide
              </button>
              {copyStatus ? <small>{copyStatus}</small> : null}
            </div>
          </div>
        ) : null}

        <div className="settings-key-list">
          {appPasswords.map((item) => (
            <div className="settings-key-row" key={item.id}>
              <div>
                <strong>{item.name}</strong>
                <span>
                  key ends in <code>{item.lastFour}</code> - created {formatDate(item.createdAt)}
                </span>
              </div>
              <button className="btn btn-ghost" type="button" onClick={() => onRevoke(item)}>
                Revoke
              </button>
            </div>
          ))}
          {appPasswords.length === 0 ? <p className="empty-state">No fixed helper keys have been created.</p> : null}
        </div>
      </div>
    </details>
  );
}
