import { FormEvent, useCallback, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiBaseForUi } from "../../services/apiClient";
import {
  createAppPassword,
  enableAutoAppPasswordEnrollment,
  getAutoAppPasswordEnrollmentStatus,
  getCurrentUser,
  listAppPasswords,
  revokeAppPassword
} from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";

export function SettingsPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [user, appPasswords, autoEnrollment] = await Promise.all([
      getCurrentUser(),
      listAppPasswords(),
      getAutoAppPasswordEnrollmentStatus()
    ]);
    return { user, appPasswords, autoEnrollment };
  }, []);
  const { loading, error, data, reload } = useAsyncData(loader, []);

  const [nameDraft, setNameDraft] = useState("");
  const [generateBusy, setGenerateBusy] = useState(false);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [copyStatus, setCopyStatus] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [enablingAutoEnrollment, setEnablingAutoEnrollment] = useState(false);

  async function onGenerate(event: FormEvent): Promise<void> {
    event.preventDefault();
    setGenerateBusy(true);
    setActionError(null);
    setCopyStatus(null);
    try {
      const response = await createAppPassword(nameDraft.trim() || undefined);
      setGeneratedKey(response.plainTextKey);
      setNameDraft("");
      reload();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Sleutel genereren mislukt");
    } finally {
      setGenerateBusy(false);
    }
  }

  async function onCopy(): Promise<void> {
    if (!generatedKey) {
      return;
    }
    try {
      await navigator.clipboard.writeText(generatedKey);
      setCopyStatus("Gekopieerd");
    } catch {
      setCopyStatus("Kopiëren niet beschikbaar in deze browser");
    }
  }

  async function onRevoke(id: string): Promise<void> {
    const confirmed = window.confirm("Deze app key intrekken?");
    if (!confirmed) {
      return;
    }
    setActionError(null);
    try {
      await revokeAppPassword(id);
      reload();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Intrekken mislukt");
    }
  }

  async function onEnableAutoEnrollment(): Promise<void> {
    setEnablingAutoEnrollment(true);
    setActionError(null);
    try {
      await enableAutoAppPasswordEnrollment(15);
      reload();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Auto password venster activeren mislukt");
    } finally {
      setEnablingAutoEnrollment(false);
    }
  }

  return (
    <SectionCard title="Settings" subtitle="Runtime en device-auth instellingen voor deze web app.">
      <p>
        <strong>API base:</strong> <code>{apiBaseForUi()}</code>
      </p>
      <p>
        <strong>Auth mode:</strong> <code>disabled</code>
      </p>
      {loading ? <LoadingState label="Settings laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {actionError ? <ErrorState message={actionError} /> : null}

      {data ? (
        <>
          <div className="stack compact">
            <p>
              <strong>User:</strong> {data.user.email}
            </p>
            <p>
              <strong>Storage used:</strong> {formatBytes(data.user.storageUsedBytes)}
            </p>
            <p>
              <strong>Games:</strong> {data.user.gameCount}
            </p>
            <p>
              <strong>Files:</strong> {data.user.fileCount}
            </p>
          </div>

          <hr className="divider" />

          <div className="stack compact">
            <h3>App Passwords</h3>
            <p>Genereer per helper/device een aparte sleutel. De sleutel wordt maar één keer volledig getoond.</p>
            <div className="inline-actions">
              <button className="btn btn-ghost" type="button" onClick={() => void onEnableAutoEnrollment()} disabled={enablingAutoEnrollment}>
                {enablingAutoEnrollment ? "Activating..." : "Enable auto passwords for 15min"}
              </button>
              <small>
                Auto-enroll:{" "}
                {data.autoEnrollment.active
                  ? `active tot ${formatDate(data.autoEnrollment.enabledUntil ?? null)}`
                  : "uitgeschakeld"}
              </small>
            </div>
          </div>

          <form className="inline-actions" onSubmit={(event) => void onGenerate(event)}>
            <input
              value={nameDraft}
              onChange={(event) => setNameDraft(event.target.value)}
              placeholder="Naam (bijv. SteamDeck)"
              aria-label="App password naam"
            />
            <button className="btn btn-primary" type="submit" disabled={generateBusy}>
              {generateBusy ? "Generating..." : "Generate"}
            </button>
          </form>

          {generatedKey ? (
            <div className="generated-key-box" role="status" aria-live="polite">
              <p>
                <strong>Nieuwe key:</strong> <code>{generatedKey}</code>
              </p>
              <div className="inline-actions">
                <button className="btn btn-ghost" type="button" onClick={() => void onCopy()}>
                  Copy
                </button>
                <button className="btn btn-ghost" type="button" onClick={() => setGeneratedKey(null)}>
                  Hide
                </button>
                {copyStatus ? <small>{copyStatus}</small> : null}
              </div>
            </div>
          ) : null}

          <div className="saves-table-wrap">
            <table className="saves-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Last 4</th>
                  <th>Bound Device</th>
                  <th>Console sync</th>
                  <th>Created</th>
                  <th>Last used</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {data.appPasswords.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>
                      <code>{item.lastFour}</code>
                    </td>
                    <td>{item.boundDeviceId ? `Device #${item.boundDeviceId}` : "Not bound"}</td>
                    <td>{item.syncAll ? "All" : (item.allowedSystemSlugs ?? []).join(", ") || "None"}</td>
                    <td>{formatDate(item.createdAt)}</td>
                    <td>{item.lastUsedAt ? formatDate(item.lastUsedAt) : "-"}</td>
                    <td>
                      <button className="btn btn-ghost" type="button" onClick={() => void onRevoke(item.id)}>
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : null}
    </SectionCard>
  );
}
