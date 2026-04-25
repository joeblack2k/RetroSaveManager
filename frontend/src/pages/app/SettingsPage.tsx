import { ChangeEvent, FormEvent, useCallback, useMemo, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiBaseForUi } from "../../services/apiClient";
import {
  createAppPassword,
  deleteGameModule,
  disableGameModule,
  enableGameModule,
  getCurrentUser,
  listAppPasswords,
  listGameModules,
  rescanGameModules,
  revokeAppPassword,
  syncGameModules,
  uploadGameModule
} from "../../services/retrosaveApi";
import type { GameModuleListResponse, GameModuleRecord } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";

export function SettingsPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [user, appPasswords, modules] = await Promise.all([getCurrentUser(), listAppPasswords(), listGameModules()]);
    return { user, appPasswords, modules };
  }, []);
  const { loading, error, data, reload } = useAsyncData(loader, []);

  const [nameDraft, setNameDraft] = useState("");
  const [generateBusy, setGenerateBusy] = useState(false);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [copyStatus, setCopyStatus] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [moduleBusy, setModuleBusy] = useState<string | null>(null);
  const [moduleFile, setModuleFile] = useState<File | null>(null);
  const [moduleMessage, setModuleMessage] = useState<string | null>(null);
  const [moduleError, setModuleError] = useState<string | null>(null);

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
      setActionError(err instanceof Error ? err.message : "Failed to generate key");
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
      setCopyStatus("Copied");
    } catch {
      setCopyStatus("Copy is not available in this browser");
    }
  }

  async function onRevoke(id: string): Promise<void> {
    const confirmed = window.confirm("Revoke this app key?");
    if (!confirmed) {
      return;
    }
    setActionError(null);
    try {
      await revokeAppPassword(id);
      reload();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Failed to revoke key");
    }
  }

  function onModuleFileChange(event: ChangeEvent<HTMLInputElement>): void {
    setModuleFile(event.target.files?.[0] ?? null);
    setModuleMessage(null);
    setModuleError(null);
  }

  async function runModuleAction(key: string, action: () => Promise<string>): Promise<void> {
    setModuleBusy(key);
    setModuleMessage(null);
    setModuleError(null);
    try {
      const message = await action();
      setModuleMessage(message);
      reload();
    } catch (err: unknown) {
      setModuleError(err instanceof Error ? err.message : "Module action failed");
    } finally {
      setModuleBusy(null);
    }
  }

  async function onSyncModules(): Promise<void> {
    await runModuleAction("sync", async () => {
      const status = await syncGameModules();
      return `Module sync finished: ${status.importedCount} imported, ${status.errorCount} validation errors.`;
    });
  }

  async function onUploadModule(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (!moduleFile) {
      setModuleError("Choose a .rsmodule.zip file first.");
      return;
    }
    await runModuleAction("upload", async () => {
      const record = await uploadGameModule(moduleFile);
      setModuleFile(null);
      return `Module ${record.manifest.moduleId} is now ${record.status}.`;
    });
  }

  async function onRescanModules(): Promise<void> {
    await runModuleAction("rescan", async () => {
      const response = await rescanGameModules();
      const scanned = typeof response.result.scanned === "number" ? response.result.scanned : undefined;
      return scanned === undefined ? "Save rescan finished." : `Save rescan finished: ${scanned} files scanned.`;
    });
  }

  async function onEnableModule(moduleId: string): Promise<void> {
    await runModuleAction(`enable:${moduleId}`, async () => {
      const record = await enableGameModule(moduleId);
      return `Module ${record.manifest.moduleId} is enabled.`;
    });
  }

  async function onDisableModule(moduleId: string): Promise<void> {
    await runModuleAction(`disable:${moduleId}`, async () => {
      const record = await disableGameModule(moduleId);
      return `Module ${record.manifest.moduleId} is disabled.`;
    });
  }

  async function onDeleteModule(moduleId: string): Promise<void> {
    if (!window.confirm(`Delete module ${moduleId}? The module can be synced or uploaded again later.`)) {
      return;
    }
    await runModuleAction(`delete:${moduleId}`, async () => {
      const record = await deleteGameModule(moduleId);
      return `Module ${record.manifest.moduleId} is marked as deleted.`;
    });
  }

  return (
    <SectionCard title="Settings" subtitle="Runtime modules and device authentication for this web app.">
      <p>
        <strong>API base:</strong> <code>{apiBaseForUi()}</code>
      </p>
      <p>
        <strong>Auth mode:</strong> <code>disabled</code>
      </p>
      {loading ? <LoadingState label="Loading settings..." /> : null}
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
            <p>Generate a separate key for each helper or device. The full key is shown only once.</p>
            <small>Use the sidebar Add helper control to open a temporary 15 minute helper window.</small>
          </div>

          <form className="inline-actions" onSubmit={(event) => void onGenerate(event)}>
            <input
              value={nameDraft}
              onChange={(event) => setNameDraft(event.target.value)}
              placeholder="Name, for example SteamDeck"
              aria-label="App password name"
            />
            <button className="btn btn-primary" type="submit" disabled={generateBusy}>
              {generateBusy ? "Generating..." : "Generate"}
            </button>
          </form>

          {generatedKey ? (
            <div className="generated-key-box" role="status" aria-live="polite">
              <p>
                <strong>New key:</strong> <code>{generatedKey}</code>
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

          <hr className="divider" />

          <GameSupportModulesPanel
            modules={data.modules}
            busyKey={moduleBusy}
            selectedFile={moduleFile}
            message={moduleMessage}
            error={moduleError}
            onFileChange={onModuleFileChange}
            onSync={() => void onSyncModules()}
            onUpload={(event) => void onUploadModule(event)}
            onRescan={() => void onRescanModules()}
            onEnable={(moduleId) => void onEnableModule(moduleId)}
            onDisable={(moduleId) => void onDisableModule(moduleId)}
            onDelete={(moduleId) => void onDeleteModule(moduleId)}
          />
        </>
      ) : null}
    </SectionCard>
  );
}

type GameSupportModulesPanelProps = {
  modules: GameModuleListResponse;
  busyKey: string | null;
  selectedFile: File | null;
  message: string | null;
  error: string | null;
  onFileChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onSync: () => void;
  onUpload: (event: FormEvent) => void;
  onRescan: () => void;
  onEnable: (moduleId: string) => void;
  onDisable: (moduleId: string) => void;
  onDelete: (moduleId: string) => void;
};

function GameSupportModulesPanel({
  modules,
  busyKey,
  selectedFile,
  message,
  error,
  onFileChange,
  onSync,
  onUpload,
  onRescan,
  onEnable,
  onDisable,
  onDelete
}: GameSupportModulesPanelProps): JSX.Element {
  const counts = useMemo(() => {
    const active = modules.modules.filter((item) => item.status === "active").length;
    const disabled = modules.modules.filter((item) => item.status === "disabled").length;
    const failed = modules.modules.filter((item) => item.status === "error" || (item.errors?.length ?? 0) > 0).length;
    return { active, disabled, failed };
  }, [modules.modules]);

  const validationErrors = [
    ...(modules.library.errors ?? []).map((item) => ({ path: item.path, message: item.message })),
    ...modules.modules.flatMap((item) => (item.errors ?? []).map((moduleError) => ({ path: item.manifest.moduleId, message: moduleError })))
  ];

  return (
    <section className="module-library" aria-label="Game Support Modules">
      <header className="module-library__header">
        <div>
          <h3>Game Support Modules</h3>
          <p>
            Load parser-backed details and cheats from sandboxed WASM modules. Modules can come from GitHub or a local
            <code>.rsmodule.zip</code> upload.
          </p>
        </div>
        <div className="module-library__actions">
          <button className="btn btn-primary" type="button" onClick={onSync} disabled={busyKey !== null}>
            {busyKey === "sync" ? "Syncing..." : "Sync from GitHub"}
          </button>
          <button className="btn btn-ghost" type="button" onClick={onRescan} disabled={busyKey !== null}>
            {busyKey === "rescan" ? "Refreshing..." : "Refresh saves"}
          </button>
        </div>
      </header>

      <div className="cheats-summary" aria-label="Module summary">
        <span>{modules.modules.length} modules</span>
        <span>{counts.active} enabled</span>
        <span>{counts.disabled} disabled</span>
        <span>{counts.failed} failed</span>
        <span>{modules.library.config.repo}@{modules.library.config.ref}</span>
        <span>{modules.library.config.path}</span>
        <span>{modules.library.lastSyncedAt ? `Last sync ${formatDate(modules.library.lastSyncedAt)}` : "Never synced"}</span>
      </div>

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

      {modules.modules.length > 0 ? (
        <div className="cheat-library-table-wrap">
          <table className="cheat-library-table module-library-table">
            <thead>
              <tr>
                <th>Module</th>
                <th>System</th>
                <th>Parser</th>
                <th>Source</th>
                <th>Status</th>
                <th>Cheats</th>
                <th>Updated</th>
                <th>Controls</th>
              </tr>
            </thead>
            <tbody>
              {modules.modules.map((item) => (
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
  onDelete: (moduleId: string) => void;
}): JSX.Element {
  const moduleId = item.manifest.moduleId;
  const busy = busyKey !== null;
  return (
    <tr>
      <td>
        <strong>{item.manifest.title}</strong>
        <span>{moduleId}</span>
        <span>v{item.manifest.version}</span>
      </td>
      <td>{item.manifest.systemSlug}</td>
      <td>
        <code>{item.manifest.parserId}</code>
      </td>
      <td>
        <span>{item.source}</span>
        {item.sourcePath ? <small>{item.sourcePath}</small> : null}
      </td>
      <td>
        <span className={`cheat-status-badge cheat-status-badge--${normalizeStatusToken(item.status)}`}>{item.status}</span>
      </td>
      <td>{item.cheatPackIds?.length ?? item.manifest.cheatPacks?.length ?? 0}</td>
      <td>{formatDate(item.updatedAt)}</td>
      <td>
        <div className="module-library-controls">
          {item.status === "active" ? (
            <button className="btn btn-ghost" type="button" onClick={() => onDisable(moduleId)} disabled={busy}>
              {busyKey === `disable:${moduleId}` ? "Disabling..." : "Disable"}
            </button>
          ) : (
            <button className="btn btn-ghost" type="button" onClick={() => onEnable(moduleId)} disabled={busy}>
              {busyKey === `enable:${moduleId}` ? "Enabling..." : "Enable"}
            </button>
          )}
          <button className="btn btn-ghost" type="button" onClick={() => onDelete(moduleId)} disabled={busy}>
            {busyKey === `delete:${moduleId}` ? "Deleting..." : "Delete"}
          </button>
        </div>
      </td>
    </tr>
  );
}

function normalizeStatusToken(value: string): string {
  const token = value.trim().toLowerCase();
  if (token === "active" || token === "disabled" || token === "deleted") {
    return token;
  }
  return "source";
}
