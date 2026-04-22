import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { deleteManySaves, deleteSave, listSaves } from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";
import {
  buildSaveDetailsHref,
  buildSaveRows,
  sortSaveRows,
  systemBadgeForSlug,
  type SaveRow,
  type SaveSortDirection,
  type SaveSortKey
} from "../../utils/saveRows";

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
  { key: "rollback", label: "Rollback" },
  { key: "date", label: "Date" }
];

export function MyGamesPage(): JSX.Element {
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingKeys, setDeletingKeys] = useState<string[]>([]);
  const [sortKey, setSortKey] = useState<SaveSortKey>(DEFAULT_SORT.key);
  const [sortDirection, setSortDirection] = useState<SaveSortDirection>(DEFAULT_SORT.direction);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});

  const loader = useCallback(async () => listSaves(), []);

  const { loading, error, data, reload } = useAsyncData(loader, []);

  const rows = useMemo<SaveRow[]>(() => {
    return buildSaveRows(data ?? []);
  }, [data]);

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

  const totalSaveCount = useMemo(() => rows.reduce((sum, row) => sum + row.saveCount, 0), [rows]);
  const totalBytes = useMemo(() => rows.reduce((sum, row) => sum + row.totalBytes, 0), [rows]);
  const summaryText = `${consoleGroups.length} ${pluralize(consoleGroups.length, "system", "systems")} · ${rows.length} ${pluralize(rows.length, "game", "games")} · ${totalSaveCount} ${pluralize(totalSaveCount, "save", "saves")} · ${formatBytes(totalBytes)} total`;

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
                          <tr
                            key={row.key}
                            className="treegrid-child-row"
                            data-treegrid-group={group.key}
                            data-treegrid-node="child"
                          >
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
                            <td>{formatCountLabel(row.saveCount, "save", "saves")}</td>
                            <td>{formatBytes(row.latestSizeBytes)}</td>
                            <td>{formatBytes(row.totalBytes)}</td>
                            <td>
                              <Link
                                className="treegrid-action treegrid-action--rollback"
                                to={detailsHref}
                                aria-label={`Rollback ${row.gameName}`}
                                title={`Rollback ${row.gameName}`}
                              >
                                <HistoryIcon />
                                <span>{formatCountLabel(row.latestVersion, "version", "versions")}</span>
                              </Link>
                            </td>
                            <td>{formatCompactDate(row.latestCreatedAt)}</td>
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
                              <a
                                className="treegrid-icon-button treegrid-icon-button--download"
                                href={row.downloadUrl}
                                aria-label={`Download ${row.gameName}`}
                                title={`Download ${row.gameName}`}
                              >
                                <DownloadIcon />
                              </a>
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
    </section>
  );
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
      return (
        <svg className="treegrid-system-glyph" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="4.5" y="6.5" width="15" height="11" rx="3" />
          <path d="M9 12h3M10.5 10.5v3" />
          <circle cx="15.5" cy="11" r="1" fill="currentColor" stroke="none" />
          <circle cx="17.5" cy="13" r="1" fill="currentColor" stroke="none" />
        </svg>
      );
    default:
      return <span className="treegrid-platform-badge__label">{fallbackLabel}</span>;
  }
}

function HistoryIcon(): JSX.Element {
  return (
    <svg className="treegrid-inline-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M4.5 8.5V4.8h3.7" />
      <path d="M5.1 12a6.9 6.9 0 1 0 2-4.9" />
      <path d="M12 8.2v4.1l2.8 1.7" />
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
