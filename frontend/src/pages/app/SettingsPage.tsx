import { type ChangeEvent, type FormEvent, useCallback, useState } from "react";
import { ConfirmDialog } from "../../components/ConfirmDialog";
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
import type { AppPassword, GameModuleRecord } from "../../services/types";
import { GameSupportModulesPanel } from "./settings/GameSupportModulesPanel";
import { HelperKeysPanel } from "./settings/HelperKeysPanel";

type PendingSettingsDelete =
  | { kind: "app-password"; key: AppPassword }
  | { kind: "module"; module: GameModuleRecord };

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
  const [pendingDelete, setPendingDelete] = useState<PendingSettingsDelete | null>(null);
  const [confirmBusy, setConfirmBusy] = useState(false);

  async function onGenerate(event: FormEvent): Promise<void> {
    event.preventDefault();
    setGenerateBusy(true);
    setActionError(null);
    setCopyStatus(null);
    try {
      const response = await createAppPassword(nameDraft.trim() || undefined);
      setGeneratedKey(response.plainTextKey);
      setNameDraft("");
      await reload();
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

  async function confirmDelete(): Promise<void> {
    if (!pendingDelete) {
      return;
    }
    setConfirmBusy(true);
    if (pendingDelete.kind === "app-password") {
      await revokePendingKey(pendingDelete.key);
      return;
    }
    await deletePendingModule(pendingDelete.module);
  }

  async function revokePendingKey(key: AppPassword): Promise<void> {
    setActionError(null);
    try {
      await revokeAppPassword(key.id);
      setPendingDelete(null);
      await reload();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Failed to revoke key");
    } finally {
      setConfirmBusy(false);
    }
  }

  async function deletePendingModule(module: GameModuleRecord): Promise<void> {
    const moduleId = module.manifest.moduleId;
    const deleted = await runModuleAction(`delete:${moduleId}`, async () => {
      const record = await deleteGameModule(moduleId);
      return `Module ${record.manifest.moduleId} is marked as deleted.`;
    });
    if (deleted) {
      setPendingDelete(null);
    }
    setConfirmBusy(false);
  }

  function onModuleFileChange(event: ChangeEvent<HTMLInputElement>): void {
    setModuleFile(event.target.files?.[0] ?? null);
    setModuleMessage(null);
    setModuleError(null);
  }

  async function runModuleAction(key: string, action: () => Promise<string>): Promise<boolean> {
    setModuleBusy(key);
    setModuleMessage(null);
    setModuleError(null);
    try {
      const message = await action();
      setModuleMessage(message);
      await reload();
      return true;
    } catch (err: unknown) {
      setModuleError(err instanceof Error ? err.message : "Module action failed");
      return false;
    } finally {
      setModuleBusy(null);
    }
  }

  async function onSyncModules(refreshSaves: boolean): Promise<void> {
    await runModuleAction(refreshSaves ? "sync-refresh" : "sync", async () => {
      const status = await syncGameModules();
      if (!refreshSaves) {
        return `Module sync finished: ${status.importedCount} imported, ${status.errorCount} validation errors.`;
      }
      const response = await rescanGameModules();
      const scanned = typeof response.result.scanned === "number" ? response.result.scanned : undefined;
      const scanText = scanned === undefined ? "saves refreshed" : `${scanned} save files refreshed`;
      return `Module sync finished: ${status.importedCount} imported, ${status.errorCount} validation errors, ${scanText}.`;
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
      const response = await rescanGameModules();
      const scanned = typeof response.result.scanned === "number" ? response.result.scanned : undefined;
      setModuleFile(null);
      return scanned === undefined
        ? `Module ${record.manifest.moduleId} is now ${record.status}. Saves refreshed.`
        : `Module ${record.manifest.moduleId} is now ${record.status}. ${scanned} save files refreshed.`;
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
      const response = await rescanGameModules();
      const scanned = typeof response.result.scanned === "number" ? response.result.scanned : undefined;
      return scanned === undefined
        ? `Module ${record.manifest.moduleId} is enabled. Saves refreshed.`
        : `Module ${record.manifest.moduleId} is enabled. ${scanned} save files refreshed.`;
    });
  }

  async function onDisableModule(moduleId: string): Promise<void> {
    await runModuleAction(`disable:${moduleId}`, async () => {
      const record = await disableGameModule(moduleId);
      return `Module ${record.manifest.moduleId} is disabled.`;
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
            onSync={() => void onSyncModules(false)}
            onSyncAndRefresh={() => void onSyncModules(true)}
            onUpload={(event) => void onUploadModule(event)}
            onRescan={() => void onRescanModules()}
            onEnable={(moduleId) => void onEnableModule(moduleId)}
            onDisable={(moduleId) => void onDisableModule(moduleId)}
            onDelete={(module) => setPendingDelete({ kind: "module", module })}
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
            onRevoke={(key) => setPendingDelete({ kind: "app-password", key })}
          />
        </div>
      ) : null}
      {pendingDelete ? (
        <ConfirmDialog
          title={pendingDelete.kind === "app-password" ? "Revoke helper key" : "Delete game module"}
          message={
            pendingDelete.kind === "app-password"
              ? `This revokes "${pendingDelete.key.name}" immediately. Any helper using the key ending in ${pendingDelete.key.lastFour} must receive a new key before it can sync again.`
              : `This marks module "${pendingDelete.module.manifest.moduleId}" as deleted. It can be synced or uploaded again later, but its parser and cheat support stop applying after refresh.`
          }
          confirmLabel={pendingDelete.kind === "app-password" ? "Revoke key" : "Delete module"}
          danger
          busy={confirmBusy}
          onConfirm={() => void confirmDelete()}
          onCancel={() => setPendingDelete(null)}
        />
      ) : null}
    </SectionCard>
  );
}
