import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import {
  applySaveCheats,
  deleteManySaves,
  deleteSave,
  getSaveCheats,
  getSaveHistory,
  listSaves,
  rollbackSave,
  uploadSaveFile
} from "../../services/retrosaveApi";
import type { SaveCheatEditorState, SaveCheatField, SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";
import {
  buildSaveDownloadHref,
  buildSaveDetailsHref,
  buildSaveRows,
  sortSaveRows,
  systemBadgeForSlug,
  type SaveRow,
  type SaveSortDirection,
  type SaveSortKey
} from "../../utils/saveRows";
import type { SaveDownloadProfile } from "../../services/types";

type ConsoleGroup = {
  key: string;
  name: string;
  rows: SaveRow[];
  saveCount: number;
  totalBytes: number;
};

const DEFAULT_SORT: { key: SaveSortKey; direction: SaveSortDirection } = {
  key: "date",
  direction: "desc"
};

const SORTABLE_COLUMNS: Array<{ key: SaveSortKey; label: string; align?: "left" | "center" }> = [
  { key: "game", label: "Gamename" },
  { key: "region", label: "Region", align: "center" },
  { key: "saves", label: "Saves" },
  { key: "latest", label: "Latest" },
  { key: "total", label: "Total" },
  { key: "date", label: "Date" }
];

type SaveSelectorState = {
  row: SaveRow;
  displayTitle: string;
  versions: SaveSummary[];
};

type DownloadModalState = {
  title: string;
  request: { saveId: string; psLogicalKey?: string; revisionId?: string };
  profiles: SaveDownloadProfile[];
};

export function MyGamesPage(): JSX.Element {
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingKeys, setDeletingKeys] = useState<string[]>([]);
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

  const loader = useCallback(async () => listSaves(), []);

  const { loading, error, data, reload } = useAsyncData(loader, []);

  const rows = useMemo<SaveRow[]>(() => buildSaveRows(data ?? []), [data]);

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
    if (!selectorState && !cheatRow && !downloadState && !uploadOpen) {
      return;
    }

    function handleEscape(event: KeyboardEvent): void {
      if (event.key === "Escape") {
        closeSaveSelector();
        closeCheatModal();
        closeDownloadModal();
        closeUploadModal();
      }
    }

    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [selectorState, cheatRow, downloadState, uploadOpen]);

  const totalSaveCount = useMemo(() => rows.reduce((sum, row) => sum + row.saveCount, 0), [rows]);
  const totalBytes = useMemo(() => rows.reduce((sum, row) => sum + row.totalBytes, 0), [rows]);
  const summaryText = `${consoleGroups.length} ${pluralize(consoleGroups.length, "system", "systems")} · ${rows.length} ${pluralize(rows.length, "game", "games")} · ${totalSaveCount} ${pluralize(totalSaveCount, "save", "saves")} · ${formatBytes(totalBytes)} total`;

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
    const saveLabel = row.saveCount === 1 ? "save" : "saves";
    const confirmed = window.confirm(`Are you sure?\n\nThis will permanently delete ${row.saveCount} ${saveLabel} for "${row.gameName}".`);
    if (!confirmed) {
      return;
    }

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
    setUploadError(null);
    setUploadResult(null);
  }

  function closeUploadModal(): void {
    setUploadOpen(false);
    setUploadFile(null);
    setUploadBusy(false);
    setUploadError(null);
    setUploadResult(null);
  }

  async function handleUploadSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (!uploadFile) {
      setUploadError("Choose a save file or zip archive first.");
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
        wiiTitleId: uploadWiiTitleId || undefined
      });
      const count = response.successCount && response.successCount > 1 ? response.successCount : 1;
      setUploadResult(`${count} ${pluralize(count, "save", "saves")} imported.`);
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

  if (loading) {
    return <LoadingState label="Loading My Saves..." />;
  }

  if (error) {
    return <ErrorState message={error} />;
  }

  return (
    <section className="treegrid-panel fade-in-up">
      <header className="treegrid-panel__header">
        <div>
          <h1>My Saves</h1>
          <p>{summaryText}</p>
        </div>
        <button className="treegrid-header-action" type="button" onClick={openUploadModal}>
          Upload
        </button>
      </header>

      {deleteError ? <p className="error-state">{deleteError}</p> : null}
      {rows.length === 0 ? <p className="treegrid-panel__empty">No saves found.</p> : null}

      {rows.length > 0 ? (
        <div className="treegrid-table-wrap">
          <table className="treegrid-table" role="treegrid" aria-label="My Saves">
            <thead>
              <tr>
                {SORTABLE_COLUMNS.map((column) => (
                  <th key={column.key} className={column.align === "center" ? "treegrid-table__align-center" : undefined}>
                    <button
                      className="treegrid-sort"
                      type="button"
                      onClick={() => handleSort(column.key)}
                      aria-label={`Sort by ${column.label}`}
                    >
                      <span>{column.label}</span>
                      <span className={`treegrid-sort__icon${sortKey === column.key ? " treegrid-sort__icon--active" : ""}`} aria-hidden="true">
                        {sortKey === column.key ? (sortDirection === "asc" ? "↑" : "↓") : "↕"}
                      </span>
                    </button>
                  </th>
                ))}
                <th>Cheats</th>
                <th>Details</th>
                <th>Download</th>
                <th>Delete</th>
              </tr>
            </thead>

            {consoleGroups.map((group) => {
              const expanded = expandedGroups[group.key] ?? false;
              const folderMeta = `${group.rows.length} ${pluralize(group.rows.length, "game", "games")} · ${group.saveCount} ${pluralize(group.saveCount, "save", "saves")}`;
              return (
                <tbody key={group.key} className="treegrid-group-body">
                  <tr className="treegrid-group-row" aria-expanded={expanded}>
                    <td colSpan={10}>
                      <button
                        className="treegrid-group-toggle"
                        type="button"
                        onClick={() => toggleGroup(group.key)}
                        aria-expanded={expanded}
                        aria-label={`${expanded ? "Collapse" : "Expand"} ${group.name}`}
                      >
                        <span className="treegrid-group-toggle__icon" aria-hidden="true">
                          <ChevronIcon expanded={expanded} />
                        </span>
                        <FolderIcon />
                        <span className="treegrid-group-toggle__title">{group.name}</span>
                        <span className="treegrid-group-toggle__meta">{folderMeta}</span>
                      </button>
                    </td>
                  </tr>

                  {expanded
                    ? group.rows.map((row) => {
                        const isDeleting = deletingKeys.includes(row.key);
                        const detailsHref = buildSaveDetailsHref(row);
                        const platformBadge = systemBadgeForSlug(row.systemSlug);
                        return (
                          <tr key={row.key} className="treegrid-child-row" data-treegrid-group={group.key} data-treegrid-node="child">
                            <td data-treegrid-cell="game">
                              <div className="treegrid-game-cell treegrid-game-cell--child">
                                <span className="treegrid-platform-badge" aria-hidden="true" title={platformBadge.title}>
                                  <SystemGlyph systemSlug={row.systemSlug} fallbackLabel={platformBadge.label} />
                                </span>
                                <span className="treegrid-game-title">{row.gameName}</span>
                              </div>
                            </td>
                            <td className="treegrid-table__align-center">
                              <span className="treegrid-region" title={row.regionCode}>
                                <span className="treegrid-region__flag" aria-hidden="true">{row.regionFlag}</span>
                                <span>{displayRegionCode(row.regionCode)}</span>
                              </span>
                            </td>
                            <td>
                              {row.saveCount > 1 ? (
                                <button
                                  className="treegrid-save-trigger"
                                  type="button"
                                  onClick={() => void handleOpenSaveSelector(row)}
                                  aria-label={`Select sync save for ${row.gameName}`}
                                  title={`Select sync save for ${row.gameName}`}
                                >
                                  {formatCountLabel(row.saveCount, "save", "saves")}
                                </button>
                              ) : (
                                <span>{formatCountLabel(row.saveCount, "save", "saves")}</span>
                              )}
                            </td>
                            <td>{formatBytes(row.latestSizeBytes)}</td>
                            <td>{formatBytes(row.totalBytes)}</td>
                            <td>{formatCompactDate(row.latestCreatedAt)}</td>
                            <td>
                              {row.cheatsSupported && row.cheatAvailableCount > 0 ? (
                                <button
                                  className="treegrid-cheat-trigger"
                                  type="button"
                                  onClick={() => void handleOpenCheats(row)}
                                  aria-label={`Edit cheats for ${row.gameName}`}
                                  title={`Edit cheats for ${row.gameName}`}
                                >
                                  <CheatIcon />
                                  <span>{formatCountLabel(row.cheatAvailableCount, "cheat", "cheats")}</span>
                                </button>
                              ) : (
                                <span className="treegrid-empty-cell" aria-hidden="true">-</span>
                              )}
                            </td>
                            <td>
                              <Link
                                className="treegrid-icon-button"
                                to={detailsHref}
                                aria-label={`View details for ${row.gameName}`}
                                title={`View details for ${row.gameName}`}
                              >
                                <DetailsIcon />
                              </Link>
                            </td>
                            <td>
                              <button
                                className="treegrid-icon-button treegrid-icon-button--download"
                                type="button"
                                onClick={() => openDownloadModal(row.gameName, row.downloadRequest, row.downloadProfiles)}
                                aria-label={`Download ${row.gameName}`}
                                title={`Download ${row.gameName}`}
                              >
                                <DownloadIcon />
                              </button>
                            </td>
                            <td>
                              <button
                                className="treegrid-icon-button treegrid-icon-button--danger"
                                type="button"
                                onClick={() => void handleDeleteRow(row)}
                                disabled={isDeleting}
                                aria-label={`Delete ${row.gameName}`}
                                title={`Delete ${row.gameName}`}
                              >
                                <DeleteIcon />
                              </button>
                            </td>
                          </tr>
                        );
                      })
                    : null}
                </tbody>
              );
            })}
          </table>
        </div>
      ) : null}

      {uploadOpen ? (
        <div className="treegrid-modal-backdrop" role="presentation" onClick={closeUploadModal}>
          <section
            className="treegrid-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="treegrid-upload-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="treegrid-modal__header">
              <div>
                <h2 id="treegrid-upload-title">Upload Save</h2>
                <p>Upload a single save file or a zip archive. The backend validates and imports it for sync.</p>
              </div>
              <button className="treegrid-modal__close" type="button" onClick={closeUploadModal} aria-label="Close upload">
                Close
              </button>
            </header>

            <form className="treegrid-upload-form" onSubmit={(event) => void handleUploadSubmit(event)}>
              <div className="treegrid-upload-grid">
                <label className="treegrid-upload-field treegrid-upload-field--wide">
                  <span>Save file or zip</span>
                  <input
                    type="file"
                    accept=".zip,.bin,.sav,.srm,.sa1,.eep,.sra,.fla,.mpk,.cpk,.mcr,.mcd,.mc,.ps2,.vms,.dci,.bkr,.bcr,.bup"
                    onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
                  />
                </label>
                <label className="treegrid-upload-field">
                  <span>System</span>
                  <select value={uploadSystem} onChange={(event) => setUploadSystem(event.target.value)}>
                    <option value="">Auto-detect when possible</option>
                    <option value="wii">Nintendo Wii</option>
                    <option value="n64">Nintendo 64</option>
                    <option value="snes">Super Nintendo</option>
                    <option value="nes">Nintendo Entertainment System</option>
                    <option value="gba">Game Boy Advance</option>
                    <option value="gameboy">Game Boy</option>
                    <option value="genesis">Genesis / Mega Drive</option>
                    <option value="master-system">Master System</option>
                    <option value="game-gear">Game Gear</option>
                    <option value="dreamcast">Dreamcast</option>
                    <option value="saturn">Saturn</option>
                    <option value="psx">PlayStation</option>
                    <option value="ps2">PlayStation 2</option>
                  </select>
                </label>
                <label className="treegrid-upload-field">
                  <span>Slot name</span>
                  <input type="text" value={uploadSlotName} onChange={(event) => setUploadSlotName(event.target.value)} placeholder="default" />
                </label>
                <label className="treegrid-upload-field">
                  <span>ROM SHA1</span>
                  <input type="text" value={uploadRomSha1} onChange={(event) => setUploadRomSha1(event.target.value)} placeholder="optional but recommended" />
                </label>
                <label className="treegrid-upload-field">
                  <span>Wii title code</span>
                  <input type="text" value={uploadWiiTitleId} onChange={(event) => setUploadWiiTitleId(event.target.value.toUpperCase())} placeholder="SB4P" maxLength={4} />
                </label>
              </div>

              <p className="treegrid-upload-hint">
                Wii zip uploads can contain <code>private/wii/title/SB4P/data.bin</code> or <code>SB4P/data.bin</code>. Raw Wii <code>data.bin</code> uploads should include the title code.
              </p>
              {uploadError ? <p className="error-state">{uploadError}</p> : null}
              {uploadResult ? <p className="treegrid-modal__status">{uploadResult}</p> : null}

              <footer className="treegrid-upload-actions">
                <button className="treegrid-select-button" type="submit" disabled={uploadBusy}>
                  {uploadBusy ? "Uploading..." : "Import Save"}
                </button>
              </footer>
            </form>
          </section>
        </div>
      ) : null}

      {downloadState ? (
        <div className="treegrid-modal-backdrop" role="presentation" onClick={closeDownloadModal}>
          <section
            className="treegrid-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="treegrid-download-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="treegrid-modal__header">
              <div>
                <h2 id="treegrid-download-title">Download Save</h2>
                <p>{downloadState.title}</p>
              </div>
              <button className="treegrid-modal__close" type="button" onClick={closeDownloadModal} aria-label="Close download options">
                Close
              </button>
            </header>

            <div className="treegrid-modal__body">
              <table className="treegrid-modal-table">
                <thead>
                  <tr>
                    <th>Profile</th>
                    <th>Extension</th>
                    <th>Notes</th>
                    <th>Download</th>
                  </tr>
                </thead>
                <tbody>
                  {downloadState.profiles.map((profile) => (
                    <tr key={profile.id}>
                      <td>{profile.label}</td>
                      <td>{profile.targetExtension || "-"}</td>
                      <td>{profile.note || "-"}</td>
                      <td>
                        <a
                          className="saves-action-link"
                          href={buildSaveDownloadHref(downloadState.request, profile.id !== "original" ? profile.id : undefined)}
                        >
                          Download
                        </a>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      ) : null}

      {selectorState ? (
        <div className="treegrid-modal-backdrop" role="presentation" onClick={closeSaveSelector}>
          <section
            className="treegrid-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="treegrid-sync-save-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="treegrid-modal__header">
              <div>
                <h2 id="treegrid-sync-save-title">Select Sync Save</h2>
                <p>{selectorState.displayTitle}</p>
              </div>
              <button className="treegrid-modal__close" type="button" onClick={closeSaveSelector} aria-label="Close save selector">
                Close
              </button>
            </header>

            <div className="treegrid-modal__body">
              {selectorError ? <p className="error-state">{selectorError}</p> : null}
              {selectorLoading ? <p className="treegrid-modal__status">Loading save history...</p> : null}
              {!selectorLoading && selectorState.versions.length === 0 ? <p className="treegrid-modal__status">No save history found.</p> : null}

              {!selectorLoading && selectorState.versions.length > 0 ? (
                <table className="treegrid-modal-table">
                  <thead>
                    <tr>
                      <th>Version</th>
                      <th>Date</th>
                      <th>Size</th>
                      <th>Status</th>
                      <th>Select</th>
                    </tr>
                  </thead>
                  <tbody>
                    {selectorState.versions.map((version, index) => {
                      const isCurrent = version.id === selectorState.row.primarySaveID || (selectorState.row.primarySaveID === "" && index === 0);
                      const isBusy = selectingVersionID === version.id;
                      return (
                        <tr key={version.id}>
                          <td>v{version.version}</td>
                          <td>{formatCompactDate(version.createdAt)}</td>
                          <td>{formatBytes(version.fileSize)}</td>
                          <td>{isCurrent ? <span className="treegrid-current-pill">Current Sync Save</span> : <span>Available</span>}</td>
                          <td>
                            <button
                              className="treegrid-select-button"
                              type="button"
                              disabled={isCurrent || isBusy}
                              onClick={() => void handleSelectSyncSave(version)}
                              aria-label={`Select version ${version.version} for sync`}
                            >
                              {isCurrent ? "Current" : isBusy ? "Selecting..." : "Select"}
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              ) : null}
            </div>
          </section>
        </div>
      ) : null}

      {cheatRow ? (
        <div className="treegrid-modal-backdrop" role="presentation" onClick={closeCheatModal}>
          <section
            className="treegrid-modal treegrid-modal--wide"
            role="dialog"
            aria-modal="true"
            aria-labelledby="treegrid-cheat-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="treegrid-modal__header">
              <div>
                <h2 id="treegrid-cheat-title">Cheat Editor</h2>
                <p>{cheatDisplayTitle || cheatRow.gameName}</p>
              </div>
              <button className="treegrid-modal__close" type="button" onClick={closeCheatModal} aria-label="Close cheat editor">
                Close
              </button>
            </header>

            <div className="treegrid-modal__body treegrid-cheat-body">
              {cheatError ? <p className="error-state">{cheatError}</p> : null}
              {cheatLoading ? <p className="treegrid-modal__status">Loading cheat options...</p> : null}
              {!cheatLoading && cheatData && !cheatData.supported ? <p className="treegrid-modal__status">No safe cheat editor is available for this save.</p> : null}

              {!cheatLoading && cheatData?.supported ? (
                <>
                  {cheatData.selector?.options && cheatData.selector.options.length > 0 ? (
                    <label className="treegrid-cheat-slot-picker">
                      <span>{cheatData.selector.label}</span>
                      <select value={cheatSelectedSlot || defaultCheatSlotId(cheatData)} onChange={(event) => {
                        setCheatSelectedSlot(event.target.value);
                        setCheatPendingUpdates({});
                      }}>
                        {cheatData.selector.options.map((option) => (
                          <option key={option.id} value={option.id}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </label>
                  ) : null}

                  {cheatData.presets && cheatData.presets.length > 0 ? (
                    <div className="treegrid-cheat-presets">
                      <p className="treegrid-cheat-presets__label">Presets</p>
                      <div className="treegrid-cheat-presets__actions">
                        {cheatData.presets.map((preset) => (
                          <button
                            key={preset.id}
                            className="treegrid-cheat-preset"
                            type="button"
                            onClick={() => handleApplyPreset(preset.updates)}
                            title={preset.description || preset.label}
                          >
                            {preset.label}
                          </button>
                        ))}
                        {Object.keys(cheatPendingUpdates).length > 0 ? (
                          <button className="treegrid-cheat-preset treegrid-cheat-preset--ghost" type="button" onClick={() => setCheatPendingUpdates({})}>
                            Reset Draft
                          </button>
                        ) : null}
                      </div>
                    </div>
                  ) : null}

                  <div className="treegrid-cheat-sections">
                    {cheatData.sections?.map((section) => (
                      <section key={section.id} className="treegrid-cheat-section">
                        <header className="treegrid-cheat-section__header">
                          <h3>{section.title}</h3>
                        </header>
                        <div className="treegrid-cheat-fields">
                          {section.fields.map((field) => {
                            const currentValue = cheatPendingUpdates[field.id] ?? cheatCurrentValues[field.id];
                            return (
                              <div key={field.id} className="treegrid-cheat-field">
                                {renderCheatField(field, currentValue, handleCheatFieldChange)}
                              </div>
                            );
                          })}
                        </div>
                      </section>
                    ))}
                  </div>

                  <footer className="treegrid-cheat-actions">
                    <button className="treegrid-select-button" type="button" onClick={handleApplyCheats} disabled={cheatApplying}>
                      {cheatApplying ? "Applying..." : "Apply Cheats"}
                    </button>
                  </footer>
                </>
              ) : null}
            </div>
          </section>
        </div>
      ) : null}
    </section>
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
          <input
            type="number"
            min={field.min}
            max={field.max}
            step={field.step ?? 1}
            value={value}
            onChange={(event) => onChange(field, Number(event.target.value || 0))}
          />
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
                      const next = event.target.checked
                        ? [...selected, bit.id]
                        : selected.filter((item) => item !== bit.id);
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

function defaultCheatSlotId(data: SaveCheatEditorState | null): string {
  return data?.selector?.options?.[0]?.id ?? "";
}

function sanitizeCheatDraftValue(field: SaveCheatField, value: unknown): unknown {
  switch (field.type) {
    case "integer":
      return typeof value === "number" && Number.isFinite(value) ? value : 0;
    case "bitmask":
      return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
    case "boolean":
      return Boolean(value);
    default:
      return value;
  }
}

function pluralize(value: number, singular: string, plural: string): string {
  return value === 1 ? singular : plural;
}

function formatCountLabel(value: number, singular: string, plural: string): string {
  return `${value} ${pluralize(value, singular, plural)}`;
}

function displayRegionCode(regionCode: string): string {
  return regionCode === "UNKNOWN" ? "Other" : regionCode;
}

function formatCompactDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return formatDate(iso);
  }

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hours}:${minutes}`;
}

function defaultDirectionFor(sortKey: SaveSortKey): SaveSortDirection {
  switch (sortKey) {
    case "game":
    case "region":
      return "asc";
    default:
      return "desc";
  }
}

function FolderIcon(): JSX.Element {
  return (
    <svg className="treegrid-folder-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M3.5 7.5a2 2 0 0 1 2-2h4l1.8 1.6H18.5a2 2 0 0 1 2 2v7.2a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z" />
    </svg>
  );
}

function ChevronIcon({ expanded }: { expanded: boolean }): JSX.Element {
  return (
    <svg className="treegrid-chevron-icon" viewBox="0 0 24 24" aria-hidden="true">
      {expanded ? <path d="m6.5 9 5.5 5.5L17.5 9" /> : <path d="m9 6.5 5.5 5.5L9 17.5" />}
    </svg>
  );
}

function SystemGlyph({ systemSlug, fallbackLabel }: { systemSlug: string; fallbackLabel: string }): JSX.Element {
  switch (systemSlug) {
    case "psx":
    case "ps2":
    case "psp":
    case "psvita":
      return (
        <svg className="treegrid-system-glyph" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M7 5.5h4.6c2.7 0 4.4 1.4 4.4 3.8 0 2.4-1.7 3.8-4.4 3.8H10v5.4H7z" />
          <path d="M12.7 12.1c3 .5 5.1 1.5 5.1 3.2 0 2.1-2.5 3.2-5.8 3.2-2 0-3.8-.3-5.2-.9" />
        </svg>
      );
    case "n64":
    case "nds":
    case "nes":
    case "snes":
    case "gameboy":
    case "gba":
    case "wii":
    case "atari-lynx":
    case "wonderswan":
      return (
        <svg className="treegrid-system-glyph" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4.5" y="6.5" width="15" height="11" rx="3" />
          <path d="M9 12h3M10.5 10.5v3" />
          <circle cx="15.5" cy="11" r="1" fill="currentColor" stroke="none" />
          <circle cx="17.5" cy="13" r="1" fill="currentColor" stroke="none" />
        </svg>
      );
    case "dreamcast":
    case "game-gear":
    case "genesis":
    case "master-system":
    case "pc-engine":
    case "sega-32x":
    case "sega-cd":
    case "sg-1000":
      return (
        <svg className="treegrid-system-glyph" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="5" y="7" width="14" height="10" rx="1" />
          <path d="M7.5 10h9M7.5 13.5h6" />
        </svg>
      );
    default:
      return <span className="treegrid-platform-badge__label">{fallbackLabel}</span>;
  }
}

function CheatIcon(): JSX.Element {
  return (
    <svg className="treegrid-inline-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6 17.2 16.8 6.4" />
      <path d="m14.9 5.2 3.9 3.9" />
      <path d="M5.2 18.8 6 17.2l1.6-.8" />
    </svg>
  );
}

function DetailsIcon(): JSX.Element {
  return (
    <svg className="treegrid-inline-icon" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="8.6" />
      <path d="M12 10v5" />
      <circle cx="12" cy="7.2" r="0.8" fill="currentColor" stroke="none" />
    </svg>
  );
}

function DownloadIcon(): JSX.Element {
  return (
    <svg className="treegrid-inline-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M12 4.5v9" />
      <path d="m8.6 10.8 3.4 3.4 3.4-3.4" />
      <rect x="5" y="16.5" width="14" height="3" rx="1" />
    </svg>
  );
}

function DeleteIcon(): JSX.Element {
  return (
    <svg className="treegrid-inline-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6.8 7.4h10.4" />
      <path d="M9.2 7.4V5.5h5.6v1.9" />
      <path d="M8.2 7.4v10.1a1 1 0 0 0 1 1h5.6a1 1 0 0 0 1-1V7.4" />
    </svg>
  );
}
