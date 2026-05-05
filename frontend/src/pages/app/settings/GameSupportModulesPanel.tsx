import { ChangeEvent, FormEvent, useMemo } from "react";
import { ErrorState } from "../../../components/LoadState";
import type { GameModuleListResponse, GameModuleRecord } from "../../../services/types";
import { formatDate } from "../../../utils/format";

type GameSupportModulesPanelProps = {
  modules: GameModuleListResponse;
  busyKey: string | null;
  selectedFile: File | null;
  message: string | null;
  error: string | null;
  onFileChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onSync: () => void;
  onSyncAndRefresh: () => void;
  onUpload: (event: FormEvent) => void;
  onRescan: () => void;
  onEnable: (moduleId: string) => void;
  onDisable: (moduleId: string) => void;
  onDelete: (module: GameModuleRecord) => void;
};

export function GameSupportModulesPanel({
  modules,
  busyKey,
  selectedFile,
  message,
  error,
  onFileChange,
  onSync,
  onSyncAndRefresh,
  onUpload,
  onRescan,
  onEnable,
  onDisable,
  onDelete
}: GameSupportModulesPanelProps): JSX.Element {
  const counts = useMemo(() => {
    const active = modules.modules.filter((item) => item.status === "active").length;
    const failed = modules.modules.filter((item) => item.status === "error" || (item.errors?.length ?? 0) > 0).length;
    const cheatPacks = modules.modules.reduce((sum, item) => sum + (item.cheatPackIds?.length ?? item.manifest.cheatPacks?.length ?? 0), 0);
    return { active, failed, cheatPacks };
  }, [modules.modules]);

  const sortedModules = useMemo(
    () =>
      [...modules.modules].sort((left, right) => {
        const systemOrder = left.manifest.systemSlug.localeCompare(right.manifest.systemSlug);
        if (systemOrder !== 0) {
          return systemOrder;
        }
        return left.manifest.title.localeCompare(right.manifest.title);
      }),
    [modules.modules]
  );

  const validationErrors = [
    ...(modules.library.errors ?? []).map((item) => ({ path: item.path, message: item.message })),
    ...modules.modules.flatMap((item) => (item.errors ?? []).map((moduleError) => ({ path: item.manifest.moduleId, message: moduleError })))
  ];

  return (
    <section className="module-library" aria-label="Game Support Modules">
      <header className="settings-hero">
        <div>
          <h3>Game Support Modules</h3>
          <p>One refresh pulls worker-made parser and cheat modules from GitHub, then updates My Saves and Details.</p>
        </div>
        <div className="settings-hero__actions">
          <button className="btn btn-primary" type="button" onClick={onSyncAndRefresh} disabled={busyKey !== null}>
            {busyKey === "sync-refresh" ? "Refreshing..." : "Sync & refresh"}
          </button>
          <button className="btn btn-ghost" type="button" onClick={onSync} disabled={busyKey !== null}>
            {busyKey === "sync" ? "Syncing..." : "Sync only"}
          </button>
          <button className="btn btn-ghost" type="button" onClick={onRescan} disabled={busyKey !== null}>
            {busyKey === "rescan" ? "Refreshing..." : "Refresh saves"}
          </button>
        </div>
      </header>

      <ol className="settings-flow" aria-label="Worker module workflow">
        <li>
          <strong>Workers publish</strong>
          <span>They add reviewed <code>.rsmodule.zip</code> bundles to the GitHub module library.</span>
        </li>
        <li>
          <strong>You refresh</strong>
          <span><code>Sync & refresh</code> imports modules and rescans saves in one safe action.</span>
        </li>
        <li>
          <strong>The app updates</strong>
          <span>My Saves gets cheat icons, and Details shows new gameplay facts when a parser matches.</span>
        </li>
      </ol>

      <div className="settings-summary-grid" aria-label="Module summary">
        <SummaryTile label="Enabled" value={String(counts.active)} tone="good" />
        <SummaryTile label="Cheat packs" value={String(counts.cheatPacks)} />
        <SummaryTile label="Failed" value={String(counts.failed)} tone={counts.failed > 0 ? "bad" : undefined} />
        <SummaryTile label="Last sync" value={modules.library.lastSyncedAt ? formatDate(modules.library.lastSyncedAt) : "Never"} />
      </div>

      {message ? <p className="success-state">{message}</p> : null}
      {error ? <ErrorState message={error} /> : null}

      {validationErrors.length > 0 ? (
        <section className="cheat-library-errors" aria-label="Module validation errors">
          <h3>Validation Errors</h3>
          <ul className="plain-list">
            {validationErrors.map((item) => (
              <li key={`${item.path}:${item.message}`}>
                <strong>{item.path}</strong>
                <span>{item.message}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {sortedModules.length > 0 ? (
        <div className="settings-module-table-wrap">
          <table className="settings-module-table">
            <thead>
              <tr>
                <th>Game</th>
                <th>Console</th>
                <th>Status</th>
                <th>Cheats</th>
                <th>Updated</th>
                <th>Manage</th>
              </tr>
            </thead>
            <tbody>
              {sortedModules.map((item) => (
                <ModuleRow
                  key={item.manifest.moduleId}
                  item={item}
                  busyKey={busyKey}
                  onEnable={onEnable}
                  onDisable={onDisable}
                  onDelete={onDelete}
                />
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p className="empty-state">No game support modules are installed yet.</p>
      )}

      <details className="settings-disclosure">
        <summary>
          <span>Advanced module upload</span>
          <small>For local module testing</small>
        </summary>
        <form className="module-upload-form" onSubmit={onUpload}>
          <label>
            Upload module zip
            <input type="file" accept=".rsmodule.zip,.zip" onChange={onFileChange} />
          </label>
          <button className="btn btn-ghost" type="submit" disabled={busyKey !== null || !selectedFile}>
            {busyKey === "upload" ? "Uploading..." : "Upload module"}
          </button>
          {selectedFile ? <small>{selectedFile.name}</small> : null}
        </form>
      </details>
    </section>
  );
}

function ModuleRow({
  item,
  busyKey,
  onEnable,
  onDisable,
  onDelete
}: {
  item: GameModuleRecord;
  busyKey: string | null;
  onEnable: (moduleId: string) => void;
  onDisable: (moduleId: string) => void;
  onDelete: (module: GameModuleRecord) => void;
}): JSX.Element {
  const moduleId = item.manifest.moduleId;
  const busy = busyKey !== null;
  return (
    <tr className="settings-module-row">
      <td>
        <div>
          <strong>{item.manifest.title}</strong>
          <span>{moduleId}</span>
        </div>
      </td>
      <td>{item.manifest.systemSlug}</td>
      <td>
        <span className={`cheat-status-badge cheat-status-badge--${normalizeStatusToken(item.status)}`}>{item.status}</span>
      </td>
      <td>{item.cheatPackIds?.length ?? item.manifest.cheatPacks?.length ?? 0}</td>
      <td>{formatDate(item.updatedAt)}</td>
      <td>
        <details className="settings-row-actions">
          <summary>Manage</summary>
          <div>
            <span>{item.source === "github" ? "GitHub module" : "Uploaded module"}</span>
            {item.sourcePath ? <small>{item.sourcePath}</small> : null}
            {item.status === "active" ? (
              <button className="btn btn-ghost" type="button" onClick={() => onDisable(moduleId)} disabled={busy}>
                {busyKey === `disable:${moduleId}` ? "Disabling..." : "Disable"}
              </button>
            ) : (
              <button className="btn btn-ghost" type="button" onClick={() => onEnable(moduleId)} disabled={busy}>
                {busyKey === `enable:${moduleId}` ? "Enabling..." : "Enable"}
              </button>
            )}
            <button className="btn btn-ghost" type="button" onClick={() => onDelete(item)} disabled={busy}>
              {busyKey === `delete:${moduleId}` ? "Deleting..." : "Delete"}
            </button>
          </div>
        </details>
      </td>
    </tr>
  );
}

function SummaryTile({ label, value, tone }: { label: string; value: string; tone?: "good" | "bad" }): JSX.Element {
  return (
    <div className={tone ? `settings-summary-tile settings-summary-tile--${tone}` : "settings-summary-tile"}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function normalizeStatusToken(value: string): string {
  const token = value.trim().toLowerCase();
  if (token === "active" || token === "disabled" || token === "deleted") {
    return token;
  }
  return "source";
}
