import { ChangeEvent, FormEvent, useCallback, useMemo, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import {
  createAppPassword,
  deleteGameModule,
  disableGameModule,
  enableGameModule,
  listAppPasswords,
  listGameModules,
  rescanGameModules,
  revokeAppPassword,
  syncGameModules,
  uploadGameModule
} from "../../services/retrosaveApi";
import type { AppPassword, GameModuleListResponse, GameModuleRecord } from "../../services/types";
import { formatDate } from "../../utils/format";

export function SettingsPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [appPasswords, modules] = await Promise.all([listAppPasswords(), listGameModules()]);
    return { appPasswords, modules };
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
    <SectionCard title="Settings" subtitle="A small control room for runtime game support.">
      {loading ? <LoadingState label="Loading settings..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {actionError ? <ErrorState message={actionError} /> : null}

      {data ? (
        <div className="settings-page">
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

          <HelperKeysPanel
            appPasswords={data.appPasswords}
            nameDraft={nameDraft}
            generatedKey={generatedKey}
            generateBusy={generateBusy}
            copyStatus={copyStatus}
            onNameChange={setNameDraft}
            onGenerate={(event) => void onGenerate(event)}
            onCopy={() => void onCopy()}
            onHideKey={() => setGeneratedKey(null)}
            onRevoke={(id) => void onRevoke(id)}
          />
        </div>
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
      <header className="settings-hero">
        <div>
          <h3>Game Support Modules</h3>
          <p>Import parser-backed gameplay details and cheats from the GitHub module library.</p>
        </div>
        <div className="settings-hero__actions">
          <button className="btn btn-primary" type="button" onClick={onSync} disabled={busyKey !== null}>
            {busyKey === "sync" ? "Syncing..." : "Sync from GitHub"}
          </button>
          <button className="btn btn-ghost" type="button" onClick={onRescan} disabled={busyKey !== null}>
            {busyKey === "rescan" ? "Refreshing..." : "Refresh saves"}
          </button>
        </div>
      </header>

      <div className="settings-summary-grid" aria-label="Module summary">
        <SummaryTile label="Modules" value={String(modules.modules.length)} />
        <SummaryTile label="Enabled" value={String(counts.active)} tone="good" />
        <SummaryTile label="Disabled" value={String(counts.disabled)} />
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

      {modules.modules.length > 0 ? (
        <div className="settings-module-list">
          {modules.modules.map((item) => (
            <ModuleCard
              key={item.manifest.moduleId}
              item={item}
              busyKey={busyKey}
              onEnable={onEnable}
              onDisable={onDisable}
              onDelete={onDelete}
            />
          ))}
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

function ModuleCard({
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
    <article className="settings-module-card">
      <div className="settings-module-card__title">
        <div>
          <strong>{item.manifest.title}</strong>
          <span>
            {item.manifest.systemSlug} - {item.cheatPackIds?.length ?? item.manifest.cheatPacks?.length ?? 0} cheat packs
          </span>
        </div>
        <span className={`cheat-status-badge cheat-status-badge--${normalizeStatusToken(item.status)}`}>{item.status}</span>
      </div>
      <dl className="settings-module-card__meta">
        <div>
          <dt>Module</dt>
          <dd>{moduleId}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{formatDate(item.updatedAt)}</dd>
        </div>
      </dl>
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
    </article>
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

function HelperKeysPanel({
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
}: {
  appPasswords: AppPassword[];
  nameDraft: string;
  generatedKey: string | null;
  generateBusy: boolean;
  copyStatus: string | null;
  onNameChange: (value: string) => void;
  onGenerate: (event: FormEvent) => void;
  onCopy: () => void;
  onHideKey: () => void;
  onRevoke: (id: string) => void;
}): JSX.Element {
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
              <button className="btn btn-ghost" type="button" onClick={() => onRevoke(item.id)}>
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

function normalizeStatusToken(value: string): string {
  const token = value.trim().toLowerCase();
  if (token === "active" || token === "disabled" || token === "deleted") {
    return token;
  }
  return "source";
}
