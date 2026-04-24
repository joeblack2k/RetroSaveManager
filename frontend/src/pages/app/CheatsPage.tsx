import { FormEvent, useCallback, useState } from "react";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import {
  createCheatPack,
  deleteCheatPack,
  disableCheatPack,
  enableCheatPack,
  listCheatAdapters,
  listCheatPacks
} from "../../services/retrosaveApi";
import type { CheatAdapterDescriptor, CheatManagedPack } from "../../services/types";
import { formatDate } from "../../utils/format";

type CheatsDashboardData = {
  packs: CheatManagedPack[];
  adapters: CheatAdapterDescriptor[];
};

const DEFAULT_UPLOAD_SOURCE = "uploaded";

export function CheatsPage(): JSX.Element {
  const loader = useCallback(
    async () =>
      ({
        packs: await listCheatPacks(),
        adapters: await listCheatAdapters()
      }) satisfies CheatsDashboardData,
    []
  );
  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [yaml, setYAML] = useState("");
  const [source, setSource] = useState(DEFAULT_UPLOAD_SOURCE);
  const [publishedBy, setPublishedBy] = useState("");
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitSuccess, setSubmitSuccess] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busyPackId, setBusyPackID] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const trimmedYAML = yaml.trim();
    if (!trimmedYAML) {
      setSubmitSuccess(null);
      setSubmitError("Pack YAML is required.");
      return;
    }
    setSubmitting(true);
    setSubmitError(null);
    setSubmitSuccess(null);
    try {
      const response = await createCheatPack({
        yaml: trimmedYAML,
        source,
        publishedBy: publishedBy.trim() || undefined,
        notes: notes.trim() || undefined
      });
      setYAML("");
      setNotes("");
      setPublishedBy("");
      setSubmitSuccess(`Pack ${response.pack.manifest.packId} is now live.`);
      reload();
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "Failed to publish cheat pack.");
    } finally {
      setSubmitting(false);
    }
  }

  async function handlePackAction(pack: CheatManagedPack, action: "enable" | "disable" | "delete"): Promise<void> {
    const packId = pack.manifest.packId;
    const needsConfirmation = action === "delete";
    if (needsConfirmation && !window.confirm(`Delete cheat pack ${packId}?`)) {
      return;
    }
    setActionError(null);
    setBusyPackID(packId);
    try {
      if (action === "enable") {
        await enableCheatPack(packId);
      } else if (action === "disable") {
        await disableCheatPack(packId);
      } else {
        await deleteCheatPack(packId);
      }
      reload();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : `Failed to ${action} cheat pack.`);
    } finally {
      setBusyPackID(null);
    }
  }

  return (
    <>
      <SectionCard
        title="Cheats"
        subtitle="Publish live YAML packs, manage runtime status, and inspect which adapters are available for save-family cheat support."
        action={
          <button className="btn btn-ghost" type="button" onClick={reload} disabled={loading}>
            Refresh
          </button>
        }
      >
        {loading ? <LoadingState label="Loading cheat packs..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {submitError ? <ErrorState message={submitError} /> : null}
        {actionError ? <ErrorState message={actionError} /> : null}
        {submitSuccess ? <p className="success-state">{submitSuccess}</p> : null}
        {data ? (
          <>
            <div className="cheats-summary">
              <span>{data.packs.length} packs</span>
              <span>{data.adapters.length} adapters</span>
              <span>{data.packs.filter((item) => item.manifest.status === "active").length} active</span>
            </div>

            <form className="cheat-upload-form" onSubmit={(event) => void handleSubmit(event)}>
              <label className="field">
                <span>Pack YAML</span>
                <textarea
                  aria-label="Pack YAML"
                  className="cheat-upload-form__textarea"
                  rows={14}
                  value={yaml}
                  onChange={(event) => setYAML(event.target.value)}
                  placeholder={buildCheatPackPlaceholder()}
                />
              </label>

              <div className="cheat-upload-form__meta">
                <label className="field">
                  <span>Source</span>
                  <select aria-label="Source" value={source} onChange={(event) => setSource(event.target.value)}>
                    <option value="uploaded">Uploaded</option>
                    <option value="worker">Worker</option>
                  </select>
                </label>

                <label className="field">
                  <span>Published by</span>
                  <input
                    aria-label="Published by"
                    type="text"
                    value={publishedBy}
                    onChange={(event) => setPublishedBy(event.target.value)}
                    placeholder="Optional name or email"
                  />
                </label>

                <label className="field">
                  <span>Notes</span>
                  <input
                    aria-label="Notes"
                    type="text"
                    value={notes}
                    onChange={(event) => setNotes(event.target.value)}
                    placeholder="Why this pack exists"
                  />
                </label>
              </div>

              <div className="inline-actions">
                <button className="btn btn-primary" type="submit" disabled={submitting}>
                  {submitting ? "Publishing..." : "Publish pack"}
                </button>
              </div>
            </form>

            <ul className="plain-list cheat-pack-list">
              {data.packs.map((item) => {
                const packId = item.manifest.packId;
                const active = item.manifest.status === "active";
                const busy = busyPackId === packId;
                return (
                  <li key={packId} className="cheat-pack-card">
                    <div className="cheat-pack-card__header">
                      <div className="cheat-pack-card__title">
                        <strong>{item.pack.title || packId}</strong>
                        <p>
                          <code>{packId}</code> · <code>{item.manifest.adapterId}</code>
                        </p>
                      </div>
                      <div className="cheat-pack-card__badges">
                        <span className={`cheat-status-badge cheat-status-badge--${normalizeCheatToken(item.manifest.status)}`}>
                          {item.manifest.status}
                        </span>
                        <span className="cheat-status-badge cheat-status-badge--source">{item.manifest.source}</span>
                        {item.builtin ? <span className="cheat-status-badge cheat-status-badge--builtin">builtin</span> : null}
                      </div>
                    </div>

                    <div className="cheat-pack-card__meta">
                      <span>System: {item.pack.systemSlug || "Unknown"}</span>
                      <span>Game: {item.pack.gameId || "Unknown"}</span>
                      <span>Save UI: {item.supportsSaveUi ? "Enabled" : "Hidden"}</span>
                      <span>Updated: {formatDate(item.manifest.updatedAt)}</span>
                      <span>Published by: {item.manifest.publishedBy || "Unknown"}</span>
                    </div>

                    {item.manifest.notes ? <p className="cheat-pack-card__notes">{item.manifest.notes}</p> : null}

                    <div className="inline-actions">
                      {active ? (
                        <button
                          className="btn btn-ghost"
                          type="button"
                          aria-label={`Disable ${packId}`}
                          onClick={() => void handlePackAction(item, "disable")}
                          disabled={busy}
                        >
                          Disable
                        </button>
                      ) : (
                        <button
                          className="btn btn-ghost"
                          type="button"
                          aria-label={`Enable ${packId}`}
                          onClick={() => void handlePackAction(item, "enable")}
                          disabled={busy}
                        >
                          Enable
                        </button>
                      )}
                      <button
                        className="btn btn-ghost btn-danger"
                        type="button"
                        aria-label={`Delete ${packId}`}
                        onClick={() => void handlePackAction(item, "delete")}
                        disabled={busy}
                      >
                        Delete
                      </button>
                    </div>
                  </li>
                );
              })}
            </ul>
          </>
        ) : null}
      </SectionCard>

      <SectionCard title="Adapters" subtitle="Compile-time adapter catalog with parser requirements, match keys, and live upload support.">
        {loading ? <LoadingState label="Loading adapters..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list cheat-adapter-list">
            {data.adapters.map((adapter) => (
              <li key={adapter.id} className="cheat-adapter-card">
                <div className="cheat-adapter-card__header">
                  <strong>{adapter.id}</strong>
                  <span className="cheat-status-badge cheat-status-badge--source">{adapter.kind}</span>
                </div>
                <div className="cheat-pack-card__meta">
                  <span>Family: {adapter.family}</span>
                  <span>System: {adapter.systemSlug}</span>
                  <span>Parser: {adapter.requiredParserId || "Any"}</span>
                  <span>Minimum level: {adapter.minimumParserLevel || "container"}</span>
                  <span>Live upload: {adapter.supportsLiveUpload ? "Yes" : "No"}</span>
                  <span>Logical saves: {adapter.supportsLogicalSaves ? "Yes" : "No"}</span>
                </div>
                <p className="cheat-adapter-card__keys">
                  Match keys: {adapter.matchKeys && adapter.matchKeys.length > 0 ? adapter.matchKeys.join(", ") : "None"}
                </p>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>
    </>
  );
}

function normalizeCheatToken(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-") || "unknown";
}

function buildCheatPackPlaceholder(): string {
  return `packId: sm64-runtime-ui
schemaVersion: 1
adapterId: sm64-eeprom
gameId: n64/super-mario-64
systemSlug: n64
title: SM64 Runtime UI
match:
  titleAliases:
    - Super Mario 64
sections:
  - id: runtime-abilities
    title: Runtime Abilities
    fields:
      - id: runtimeWingCap
        ref: haveWingCap
        label: Runtime Wing Cap
        type: boolean`;
}
