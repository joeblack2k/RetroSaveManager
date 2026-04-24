import { FormEvent, useCallback, useMemo, useState } from "react";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import {
  createCheatPack,
  deleteCheatPack,
  disableCheatPack,
  enableCheatPack,
  getCheatLibrary,
  listCheatAdapters,
  listCheatPacks,
  syncCheatLibrary
} from "../../services/retrosaveApi";
import type { CheatAdapterDescriptor, CheatLibraryStatus, CheatManagedPack } from "../../services/types";

type CheatsDashboardData = {
  packs: CheatManagedPack[];
  adapters: CheatAdapterDescriptor[];
  library: CheatLibraryStatus;
};

const DEFAULT_UPLOAD_SOURCE = "uploaded";

export function CheatsPage(): JSX.Element {
  const loader = useCallback(
    async () =>
      ({
        packs: await listCheatPacks(),
        adapters: await listCheatAdapters(),
        library: await getCheatLibrary()
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
  const [syncing, setSyncing] = useState(false);
  const [syncError, setSyncError] = useState<string | null>(null);
  const [syncSuccess, setSyncSuccess] = useState<string | null>(null);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const groupedPacks = useMemo(() => groupPacksBySystem(data?.packs ?? []), [data?.packs]);
  const activeCount = data?.packs.filter((item) => item.manifest.status === "active").length ?? 0;
  const disabledCount = data?.packs.filter((item) => item.manifest.status === "disabled").length ?? 0;
  const deletedCount = data?.packs.filter((item) => item.manifest.status === "deleted").length ?? 0;

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

  async function handleLibrarySync(): Promise<void> {
    setSyncing(true);
    setSyncError(null);
    setSyncSuccess(null);
    try {
      const status = await syncCheatLibrary();
      setSyncSuccess(`GitHub sync finished: ${status.importedCount} imported, ${status.errorCount} validation errors.`);
      reload();
    } catch (err) {
      setSyncError(err instanceof Error ? err.message : "Failed to sync the cheat library.");
    } finally {
      setSyncing(false);
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
        title="Cheat Library"
        subtitle="Add YAML packs to GitHub, then sync them into this server. My Saves uses the active packs automatically."
        action={
          <button className="btn btn-ghost" type="button" onClick={reload} disabled={loading}>
            Refresh
          </button>
        }
      >
        {loading ? <LoadingState label="Loading cheat library..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {syncError ? <ErrorState message={syncError} /> : null}
        {actionError ? <ErrorState message={actionError} /> : null}
        {syncSuccess ? <p className="success-state">{syncSuccess}</p> : null}
        {data ? (
          <div className="cheat-library">
            <div className="cheats-summary" aria-label="Cheat library summary">
              <span>{data.packs.length} packs</span>
              <span>{activeCount} active</span>
              <span>{disabledCount} disabled</span>
              <span>{deletedCount} deleted</span>
              <span>{data.library.errorCount} validation errors</span>
              <span>{data.library.config.repo}@{data.library.config.ref}</span>
              <span>{data.library.config.path}</span>
              <span>Last sync: {formatCheatDate(data.library.lastSyncedAt)}</span>
            </div>

            <div className="cheat-library__hero">
              <div>
                <h3>GitHub-backed packs</h3>
                <p>
                  Put YAML files in <code>{data.library.config.path}</code>, commit them to <code>{data.library.config.repo}</code>, then sync here.
                </p>
              </div>
              <button className="btn btn-primary" type="button" onClick={() => void handleLibrarySync()} disabled={syncing}>
                {syncing ? "Syncing..." : "Sync from GitHub"}
              </button>
            </div>

            {data.library.errors.length > 0 ? (
              <section className="cheat-library-errors" aria-label="Validation errors">
                <h3>Validation Errors</h3>
                <ul className="plain-list">
                  {data.library.errors.map((item) => (
                    <li key={`${item.path}-${item.message}`}>
                      <strong>{item.path}</strong>
                      <span>{item.message}</span>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {groupedPacks.length > 0 ? (
              <div className="cheat-library-groups">
                {groupedPacks.map((group) => (
                  <section key={group.systemSlug} className="cheat-library-group">
                    <header>
                      <h3>{systemLabel(group.systemSlug)}</h3>
                      <span>{group.packs.length} packs</span>
                    </header>
                    <div className="cheat-library-table-wrap">
                      <table className="cheat-library-table">
                        <thead>
                          <tr>
                            <th>Game</th>
                            <th>Status</th>
                            <th>Source</th>
                            <th>Adapter</th>
                            <th>Updated</th>
                            <th>Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {group.packs.map((item) => renderPackRow(item, busyPackId, handlePackAction))}
                        </tbody>
                      </table>
                    </div>
                  </section>
                ))}
              </div>
            ) : (
              <p className="empty-state">No cheat packs are available yet.</p>
            )}
          </div>
        ) : null}
      </SectionCard>

      <SectionCard title="Advanced" subtitle="Manual YAML upload and adapter details are still available when you need them.">
        <button
          className="btn btn-ghost"
          type="button"
          aria-expanded={advancedOpen}
          onClick={() => setAdvancedOpen((value) => !value)}
        >
          {advancedOpen ? "Hide advanced tools" : "Show advanced tools"}
        </button>
        {advancedOpen && data ? (
          <div className="cheat-advanced-panel">
            {submitError ? <ErrorState message={submitError} /> : null}
            {submitSuccess ? <p className="success-state">{submitSuccess}</p> : null}
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

            <section className="cheat-adapter-catalog" aria-label="Adapter catalog">
              <h3>Adapter Catalog</h3>
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
            </section>
          </div>
        ) : null}
      </SectionCard>
    </>
  );
}

function renderPackRow(
  item: CheatManagedPack,
  busyPackId: string | null,
  onAction: (pack: CheatManagedPack, action: "enable" | "disable" | "delete") => Promise<void>
): JSX.Element {
  const packId = item.manifest.packId;
  const active = item.manifest.status === "active";
  const busy = busyPackId === packId;
  return (
    <tr key={packId}>
      <td>
        <strong>{item.pack.title || packId}</strong>
        <span><code>{packId}</code></span>
        {item.manifest.sourcePath ? <span>{item.manifest.sourcePath}</span> : null}
      </td>
      <td>
        <span className={`cheat-status-badge cheat-status-badge--${normalizeCheatToken(item.manifest.status)}`}>
          {item.manifest.status}
        </span>
      </td>
      <td>
        <span className="cheat-status-badge cheat-status-badge--source">{item.manifest.source}</span>
        {item.builtin ? <span className="cheat-status-badge cheat-status-badge--builtin">builtin</span> : null}
      </td>
      <td><code>{item.manifest.adapterId}</code></td>
      <td>{formatCheatDate(item.manifest.lastSyncedAt || item.manifest.updatedAt)}</td>
      <td>
        <div className="inline-actions">
          {active ? (
            <button
              className="btn btn-ghost"
              type="button"
              aria-label={`Disable ${packId}`}
              onClick={() => void onAction(item, "disable")}
              disabled={busy}
            >
              Disable
            </button>
          ) : (
            <button
              className="btn btn-ghost"
              type="button"
              aria-label={`Enable ${packId}`}
              onClick={() => void onAction(item, "enable")}
              disabled={busy}
            >
              Enable
            </button>
          )}
          <button
            className="btn btn-ghost btn-danger"
            type="button"
            aria-label={`Delete ${packId}`}
            onClick={() => void onAction(item, "delete")}
            disabled={busy}
          >
            Delete
          </button>
        </div>
      </td>
    </tr>
  );
}

function groupPacksBySystem(packs: CheatManagedPack[]): Array<{ systemSlug: string; packs: CheatManagedPack[] }> {
  const groups = new Map<string, CheatManagedPack[]>();
  for (const pack of packs) {
    const systemSlug = pack.pack.systemSlug || "unknown";
    groups.set(systemSlug, [...(groups.get(systemSlug) ?? []), pack]);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => systemLabel(a).localeCompare(systemLabel(b)))
    .map(([systemSlug, items]) => ({
      systemSlug,
      packs: items.slice().sort((a, b) => (a.pack.title || a.manifest.packId).localeCompare(b.pack.title || b.manifest.packId))
    }));
}

function systemLabel(slug: string): string {
  const labels: Record<string, string> = {
    n64: "Nintendo 64",
    snes: "Super Nintendo",
    nes: "Nintendo Entertainment System",
    gba: "Game Boy Advance",
    gameboy: "Game Boy",
    genesis: "Sega Genesis",
    dreamcast: "Sega Dreamcast"
  };
  return labels[slug] ?? slug.toUpperCase();
}

function normalizeCheatToken(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-") || "unknown";
}

function formatCheatDate(iso: string | null | undefined): string {
  if (!iso) {
    return "-";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(date);
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
