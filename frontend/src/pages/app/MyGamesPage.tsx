import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import {
  applySaveCheats,
  deleteManySaves,
  deleteSave,
  getSaveCheats,
  getSaveHistory,
  listSaves,
  previewSaveFile,
  rollbackSave,
  uploadSaveFile
} from "../../services/retrosaveApi";
import type { SaveCheatEditorState, SaveCheatField, SaveDownloadProfile, SaveSummary, SaveUploadPreviewItem } from "../../services/types";
import { formatBytes } from "../../utils/format";
import {
  buildSaveRows,
  sortSaveRows,
  type SaveRow,
  type SaveSortDirection,
  type SaveSortKey
} from "../../utils/saveRows";
import { CheatEditorDialog } from "./my-games/CheatEditorDialog";
import { DownloadSaveDialog } from "./my-games/DownloadSaveDialog";
import { MyGamesLibraryTable } from "./my-games/MyGamesLibraryTable";
import { SaveVersionSelectorDialog } from "./my-games/SaveVersionSelectorDialog";
import { UploadSaveDialog } from "./my-games/UploadSaveDialog";
import { defaultCheatSlotId, defaultDirectionFor, pluralize, sanitizeCheatDraftValue } from "./my-games/helpers";
import type { ConsoleGroup, DownloadModalState, SaveSelectorState } from "./my-games/types";

const DEFAULT_SORT: { key: SaveSortKey; direction: SaveSortDirection } = {
  key: "date",
  direction: "desc"
};

const SAVE_PAGE_SIZE = 100;

type MyGamesPageProps = {
  title?: string;
  systemSlugFilter?: string;
  emptyLabel?: string;
};

export function MyGamesPage({ title = "My Saves", systemSlugFilter, emptyLabel = "No saves found." }: MyGamesPageProps = {}): JSX.Element {
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingKeys, setDeletingKeys] = useState<string[]>([]);
  const [pendingDeleteRow, setPendingDeleteRow] = useState<SaveRow | null>(null);
  const [sortKey, setSortKey] = useState<SaveSortKey>(DEFAULT_SORT.key);
  const [sortDirection, setSortDirection] = useState<SaveSortDirection>(DEFAULT_SORT.direction);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});
  const [selectorState, setSelectorState] = useState<SaveSelectorState | null>(null);
  const [selectorLoading, setSelectorLoading] = useState(false);
  const [selectorError, setSelectorError] = useState<string | null>(null);
  const [selectingVersionID, setSelectingVersionID] = useState<string | null>(null);
  const [downloadState, setDownloadState] = useState<DownloadModalState | null>(null);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadSystem, setUploadSystem] = useState("");
  const [uploadSlotName, setUploadSlotName] = useState("");
  const [uploadRomSha1, setUploadRomSha1] = useState("");
  const [uploadWiiTitleId, setUploadWiiTitleId] = useState("");
  const [uploadRuntimeProfile, setUploadRuntimeProfile] = useState("");
  const [uploadPreviewItems, setUploadPreviewItems] = useState<SaveUploadPreviewItem[] | null>(null);
  const [uploadPreviewBusy, setUploadPreviewBusy] = useState(false);
  const [uploadBusy, setUploadBusy] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadResult, setUploadResult] = useState<string | null>(null);
  const [cheatRow, setCheatRow] = useState<SaveRow | null>(null);
  const [cheatDisplayTitle, setCheatDisplayTitle] = useState("");
  const [cheatData, setCheatData] = useState<SaveCheatEditorState | null>(null);
  const [cheatLoading, setCheatLoading] = useState(false);
  const [cheatError, setCheatError] = useState<string | null>(null);
  const [cheatApplying, setCheatApplying] = useState(false);
  const [cheatSelectedSlot, setCheatSelectedSlot] = useState("");
  const [cheatPendingUpdates, setCheatPendingUpdates] = useState<Record<string, unknown>>({});
  const [saveLimit, setSaveLimit] = useState(SAVE_PAGE_SIZE);

  const loader = useCallback(async () => listSaves({ limit: saveLimit, offset: 0, systemSlug: systemSlugFilter }), [saveLimit, systemSlugFilter]);

  const { loading, error, data, reload } = useAsyncData(loader, [saveLimit, systemSlugFilter]);

  const loadedSaves = data?.saves ?? [];
  const totalAvailableSaves = data?.total ?? loadedSaves.length;
  const rows = useMemo<SaveRow[]>(() => buildSaveRows(loadedSaves), [loadedSaves]);

  const consoleGroups = useMemo<ConsoleGroup[]>(() => {
    const grouped = new Map<string, ConsoleGroup>();
    for (const row of rows) {
      const existing = grouped.get(row.systemSlug);
      if (!existing) {
        grouped.set(row.systemSlug, {
          key: row.systemSlug,
          name: row.systemName,
          rows: [row],
          saveCount: row.saveCount,
          totalBytes: row.totalBytes
        });
        continue;
      }
      existing.rows.push(row);
      existing.saveCount += row.saveCount;
      existing.totalBytes += row.totalBytes;
    }

    const groups = [...grouped.values()];
    for (const group of groups) {
      group.rows = sortSaveRows(group.rows, sortKey, sortDirection);
    }
    groups.sort((left, right) => {
      if (left.name === "Other") {
        return 1;
      }
      if (right.name === "Other") {
        return -1;
      }
      return left.name.localeCompare(right.name);
    });
    return groups;
  }, [rows, sortDirection, sortKey]);

  useEffect(() => {
    if (consoleGroups.length === 0) {
      return;
    }
    setExpandedGroups((current) => {
      const next: Record<string, boolean> = {};
      const currentKeys = Object.keys(current);
      let changed = currentKeys.length !== consoleGroups.length;

      consoleGroups.forEach((group, index) => {
        const hasValue = Object.prototype.hasOwnProperty.call(current, group.key);
        next[group.key] = hasValue ? current[group.key] : currentKeys.length === 0 && index === 0;
        if (!hasValue) {
          changed = true;
        }
      });

      for (const key of currentKeys) {
        if (!Object.prototype.hasOwnProperty.call(next, key)) {
          changed = true;
          break;
        }
      }

      return changed ? next : current;
    });
  }, [consoleGroups]);

  useEffect(() => {
    if (!selectorState && !cheatRow && !downloadState && !uploadOpen && !pendingDeleteRow) {
      return;
    }

    function handleEscape(event: KeyboardEvent): void {
      if (event.key === "Escape") {
        closeSaveSelector();
        closeCheatModal();
        closeDownloadModal();
        closeUploadModal();
        setPendingDeleteRow(null);
      }
    }

    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [selectorState, cheatRow, downloadState, uploadOpen, pendingDeleteRow]);

  const totalSaveCount = useMemo(() => rows.reduce((sum, row) => sum + row.saveCount, 0), [rows]);
  const totalBytes = useMemo(() => rows.reduce((sum, row) => sum + row.totalBytes, 0), [rows]);
  const loadingMore = loading && loadedSaves.length > 0;
  const hasMoreSaves = loadedSaves.length < totalAvailableSaves;
  const loadedText =
    totalAvailableSaves > loadedSaves.length
      ? ` · showing ${loadedSaves.length} of ${totalAvailableSaves} library rows`
      : "";
  const summaryText = `${consoleGroups.length} ${pluralize(consoleGroups.length, "system", "systems")} · ${rows.length} ${pluralize(rows.length, "game", "games")} · ${totalSaveCount} ${pluralize(totalSaveCount, "save", "saves")} · ${formatBytes(totalBytes)} total${loadedText}`;

  const cheatCurrentValues = useMemo<Record<string, unknown>>(() => {
    if (!cheatData) {
      return {};
    }
    if (cheatData.selector?.options && cheatData.selector.options.length > 0) {
      const slotID = cheatSelectedSlot || defaultCheatSlotId(cheatData);
      return cheatData.slotValues?.[slotID] ?? {};
    }
    return cheatData.values ?? {};
  }, [cheatData, cheatSelectedSlot]);

  async function handleDeleteRow(row: SaveRow): Promise<void> {
    setDeleteError(null);
    setDeletingKeys((current) => (current.includes(row.key) ? current : [...current, row.key]));
    try {
      if (row.psLogicalKey) {
        await deleteSave(row.primarySaveID, { psLogicalKey: row.psLogicalKey });
      } else {
        await deleteManySaves(row.saveIDs);
      }
      await reload();
    } catch (err: unknown) {
      setDeleteError(err instanceof Error ? err.message : "Delete failed.");
    } finally {
      setDeletingKeys((current) => current.filter((key) => key !== row.key));
      setPendingDeleteRow(null);
    }
  }

  function toggleGroup(groupKey: string): void {
    setExpandedGroups((current) => ({
      ...current,
      [groupKey]: !current[groupKey]
    }));
  }

  function handleSort(nextKey: SaveSortKey): void {
    setSortKey((currentKey) => {
      if (currentKey === nextKey) {
        setSortDirection((currentDirection) => (currentDirection === "asc" ? "desc" : "asc"));
        return currentKey;
      }
      setSortDirection(defaultDirectionFor(nextKey));
      return nextKey;
    });
  }

  async function handleOpenSaveSelector(row: SaveRow): Promise<void> {
    if (row.saveCount <= 1) {
      return;
    }

    setSelectorState({ row, displayTitle: row.gameName, versions: [] });
    setSelectorLoading(true);
    setSelectorError(null);
    setSelectingVersionID(null);

    try {
      const history = await getSaveHistory({
        saveId: row.primarySaveID,
        psLogicalKey: row.psLogicalKey
      });
      setSelectorState({
        row,
        displayTitle: history.displayTitle?.trim() || row.gameName,
        versions: history.versions
      });
    } catch (err: unknown) {
      setSelectorError(err instanceof Error ? err.message : "Could not load save history.");
    } finally {
      setSelectorLoading(false);
    }
  }

  function closeSaveSelector(): void {
    setSelectorState(null);
    setSelectorError(null);
    setSelectorLoading(false);
    setSelectingVersionID(null);
  }

  function openDownloadModal(title: string, request: { saveId: string; psLogicalKey?: string; revisionId?: string }, profiles: SaveDownloadProfile[] | undefined): void {
    const normalized = profiles && profiles.length > 0 ? profiles : [{ id: "original", label: "Original file" }];
    setDownloadState({ title, request, profiles: normalized });
  }

  function closeDownloadModal(): void {
    setDownloadState(null);
  }

  function openUploadModal(): void {
    setUploadOpen(true);
    setUploadFile(null);
    setUploadSystem("");
    setUploadSlotName("");
    setUploadRomSha1("");
    setUploadWiiTitleId("");
    setUploadRuntimeProfile("");
    setUploadPreviewItems(null);
    setUploadPreviewBusy(false);
    setUploadError(null);
    setUploadResult(null);
  }

  function closeUploadModal(): void {
    setUploadOpen(false);
    setUploadFile(null);
    setUploadRuntimeProfile("");
    setUploadBusy(false);
    setUploadPreviewBusy(false);
    setUploadPreviewItems(null);
    setUploadError(null);
    setUploadResult(null);
  }

  function resetUploadPreview(): void {
    setUploadPreviewItems(null);
    setUploadResult(null);
    setUploadError(null);
  }

  async function handleUploadPreview(): Promise<void> {
    if (!uploadFile) {
      setUploadError("Choose a save file or zip archive first.");
      return;
    }

    setUploadPreviewBusy(true);
    setUploadError(null);
    setUploadResult(null);
    try {
      const response = await previewSaveFile({
        file: uploadFile,
        system: uploadSystem || undefined,
        slotName: uploadSlotName || undefined,
        romSha1: uploadRomSha1 || undefined,
        wiiTitleId: uploadWiiTitleId || undefined,
        runtimeProfile: uploadRuntimeProfile || undefined
      });
      setUploadPreviewItems(response.items);
      setUploadResult(`${response.acceptedCount} accepted · ${response.rejectedCount} rejected`);
    } catch (err: unknown) {
      setUploadPreviewItems(null);
      setUploadError(err instanceof Error ? err.message : "Preview failed.");
    } finally {
      setUploadPreviewBusy(false);
    }
  }

  async function handleUploadSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (!uploadFile) {
      setUploadError("Choose a save file or zip archive first.");
      return;
    }
    const acceptedCount = uploadPreviewItems?.filter((item) => item.accepted).length ?? 0;
    if (!uploadPreviewItems) {
      setUploadError("Run Preview first so you can see what will be imported or quarantined.");
      return;
    }
    if (acceptedCount === 0) {
      setUploadError("No validated saves are ready to import.");
      return;
    }

    setUploadBusy(true);
    setUploadError(null);
    setUploadResult(null);
    try {
      const response = await uploadSaveFile({
        file: uploadFile,
        system: uploadSystem || undefined,
        slotName: uploadSlotName || undefined,
        romSha1: uploadRomSha1 || undefined,
        wiiTitleId: uploadWiiTitleId || undefined,
        runtimeProfile: uploadRuntimeProfile || undefined
      });
      const count = response.successCount && response.successCount > 1 ? response.successCount : 1;
      const rejected = response.errorCount && response.errorCount > 0 ? ` · ${response.errorCount} quarantined/rejected` : "";
      setUploadResult(`${count} ${pluralize(count, "save", "saves")} imported${rejected}.`);
      await reload();
    } catch (err: unknown) {
      setUploadError(err instanceof Error ? err.message : "Upload failed.");
    } finally {
      setUploadBusy(false);
    }
  }

  async function handleSelectSyncSave(version: SaveSummary): Promise<void> {
    if (!selectorState) {
      return;
    }

    setSelectorError(null);
    setSelectingVersionID(version.id);
    try {
      if (selectorState.row.psLogicalKey) {
        await rollbackSave({
          saveId: selectorState.row.primarySaveID,
          psLogicalKey: selectorState.row.psLogicalKey,
          revisionId: version.id
        });
      } else {
        await rollbackSave({ saveId: version.id });
      }
      closeSaveSelector();
      await reload();
    } catch (err: unknown) {
      setSelectorError(err instanceof Error ? err.message : "Could not select the sync save.");
    } finally {
      setSelectingVersionID(null);
    }
  }

  async function handleOpenCheats(row: SaveRow): Promise<void> {
    if (!row.cheatsSupported || row.cheatAvailableCount <= 0) {
      return;
    }

    setCheatRow(row);
    setCheatDisplayTitle(row.gameName);
    setCheatData(null);
    setCheatLoading(true);
    setCheatError(null);
    setCheatApplying(false);
    setCheatSelectedSlot("");
    setCheatPendingUpdates({});

    try {
      const response = await getSaveCheats(row.primarySaveID, row.psLogicalKey);
      setCheatDisplayTitle(response.displayTitle?.trim() || row.gameName);
      setCheatData(response.cheats);
      setCheatSelectedSlot(defaultCheatSlotId(response.cheats));
    } catch (err: unknown) {
      setCheatError(err instanceof Error ? err.message : "Could not load cheats.");
    } finally {
      setCheatLoading(false);
    }
  }

  function closeCheatModal(): void {
    setCheatRow(null);
    setCheatDisplayTitle("");
    setCheatData(null);
    setCheatLoading(false);
    setCheatError(null);
    setCheatApplying(false);
    setCheatSelectedSlot("");
    setCheatPendingUpdates({});
  }

  function handleCheatFieldChange(field: SaveCheatField, value: unknown): void {
    setCheatPendingUpdates((current) => ({
      ...current,
      [field.id]: sanitizeCheatDraftValue(field, value)
    }));
  }

  function handleApplyPreset(updates: Record<string, unknown> | undefined): void {
    if (!updates) {
      return;
    }
    setCheatPendingUpdates((current) => ({
      ...current,
      ...updates
    }));
  }

  async function handleApplyCheats(): Promise<void> {
    if (!cheatRow || !cheatData?.supported || !cheatData.editorId) {
      return;
    }
    const selectorOptions = cheatData.selector?.options ?? [];
    const selectedSlot = selectorOptions.length > 0 ? cheatSelectedSlot || defaultCheatSlotId(cheatData) : undefined;
    if (selectorOptions.length > 0 && !selectedSlot) {
      setCheatError("Select a save file first.");
      return;
    }
    if (Object.keys(cheatPendingUpdates).length === 0) {
      setCheatError("Choose at least one cheat change before applying.");
      return;
    }

    setCheatApplying(true);
    setCheatError(null);
    try {
      await applySaveCheats({
        saveId: cheatRow.primarySaveID,
        psLogicalKey: cheatRow.psLogicalKey,
        editorId: cheatData.editorId,
        slotId: selectedSlot,
        updates: cheatPendingUpdates
      });
      closeCheatModal();
      await reload();
    } catch (err: unknown) {
      setCheatError(err instanceof Error ? err.message : "Could not apply cheats.");
    } finally {
      setCheatApplying(false);
    }
  }

  if (loading && loadedSaves.length === 0) {
    return <LoadingState label={`Loading ${title}...`} />;
  }

  if (error) {
    return <ErrorState message={error} />;
  }

  return (
    <section className="treegrid-panel fade-in-up">
      <header className="treegrid-panel__header">
        <div>
          <h1>{title}</h1>
          <p>{summaryText}</p>
        </div>
        <button className="treegrid-header-action" type="button" onClick={openUploadModal}>
          Upload
        </button>
      </header>

      {deleteError ? <p className="error-state">{deleteError}</p> : null}
      {loadingMore ? <p className="treegrid-panel__status">Refreshing save library...</p> : null}
      {rows.length === 0 ? <p className="treegrid-panel__empty">{emptyLabel}</p> : null}

      {rows.length > 0 ? (
        <MyGamesLibraryTable
          consoleGroups={consoleGroups}
          expandedGroups={expandedGroups}
          deletingKeys={deletingKeys}
          sortKey={sortKey}
          sortDirection={sortDirection}
          onToggleGroup={toggleGroup}
          onSort={handleSort}
          onOpenSaveSelector={(row) => void handleOpenSaveSelector(row)}
          onOpenCheats={(row) => void handleOpenCheats(row)}
          onOpenDownload={openDownloadModal}
          onRequestDelete={setPendingDeleteRow}
        />
      ) : null}

      {hasMoreSaves ? (
        <footer className="treegrid-pagination" aria-label="Save library pagination">
          <span>
            Showing {loadedSaves.length} of {totalAvailableSaves}
          </span>
          <button
            className="treegrid-select-button"
            type="button"
            disabled={loading}
            onClick={() => setSaveLimit((current) => current + SAVE_PAGE_SIZE)}
          >
            {loading ? "Loading..." : "Load more"}
          </button>
        </footer>
      ) : null}

      {pendingDeleteRow ? (
        <ConfirmDialog
          title="Delete saves permanently?"
          message={`This will permanently delete ${pendingDeleteRow.saveCount} ${pluralize(pendingDeleteRow.saveCount, "save", "saves")} for "${pendingDeleteRow.gameName}" from this server.`}
          confirmLabel="Delete saves"
          danger
          busy={deletingKeys.includes(pendingDeleteRow.key)}
          onConfirm={() => void handleDeleteRow(pendingDeleteRow)}
          onCancel={() => setPendingDeleteRow(null)}
        />
      ) : null}

      {uploadOpen ? (
        <UploadSaveDialog
          uploadSystem={uploadSystem}
          uploadSlotName={uploadSlotName}
          uploadRomSha1={uploadRomSha1}
          uploadWiiTitleId={uploadWiiTitleId}
          uploadRuntimeProfile={uploadRuntimeProfile}
          uploadPreviewItems={uploadPreviewItems}
          uploadPreviewBusy={uploadPreviewBusy}
          uploadBusy={uploadBusy}
          uploadError={uploadError}
          uploadResult={uploadResult}
          onClose={closeUploadModal}
          onSubmit={(event) => void handleUploadSubmit(event)}
          onPreview={() => void handleUploadPreview()}
          onFileChange={(file) => {
            setUploadFile(file);
            resetUploadPreview();
          }}
          onSystemChange={(value) => {
            setUploadSystem(value);
            setUploadRuntimeProfile("");
            resetUploadPreview();
          }}
          onRuntimeProfileChange={(value) => {
            setUploadRuntimeProfile(value);
            resetUploadPreview();
          }}
          onSlotNameChange={(value) => {
            setUploadSlotName(value);
            resetUploadPreview();
          }}
          onRomSha1Change={(value) => {
            setUploadRomSha1(value);
            resetUploadPreview();
          }}
          onWiiTitleIdChange={(value) => {
            setUploadWiiTitleId(value);
            resetUploadPreview();
          }}
        />
      ) : null}

      {downloadState ? <DownloadSaveDialog state={downloadState} onClose={closeDownloadModal} /> : null}

      {selectorState ? (
        <SaveVersionSelectorDialog
          state={selectorState}
          loading={selectorLoading}
          error={selectorError}
          selectingVersionID={selectingVersionID}
          onClose={closeSaveSelector}
          onSelect={(version) => void handleSelectSyncSave(version)}
        />
      ) : null}

      {cheatRow ? (
        <CheatEditorDialog
          row={cheatRow}
          displayTitle={cheatDisplayTitle}
          data={cheatData}
          loading={cheatLoading}
          error={cheatError}
          applying={cheatApplying}
          selectedSlot={cheatSelectedSlot}
          pendingUpdates={cheatPendingUpdates}
          currentValues={cheatCurrentValues}
          onClose={closeCheatModal}
          onSlotChange={(slotId) => {
            setCheatSelectedSlot(slotId);
            setCheatPendingUpdates({});
          }}
          onResetDraft={() => setCheatPendingUpdates({})}
          onApplyPreset={handleApplyPreset}
          onFieldChange={handleCheatFieldChange}
          onApply={handleApplyCheats}
        />
      ) : null}
    </section>
  );
}
