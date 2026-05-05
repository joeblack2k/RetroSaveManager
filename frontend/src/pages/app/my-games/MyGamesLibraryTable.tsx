import { Link } from "react-router-dom";
import type { SaveDownloadProfile } from "../../../services/types";
import { formatBytes } from "../../../utils/format";
import {
  buildSaveDetailsHref,
  systemBadgeForSlug,
  type SaveRow,
  type SaveSortDirection,
  type SaveSortKey
} from "../../../utils/saveRows";
import { CheatIcon, ChevronIcon, DeleteIcon, DetailsIcon, DownloadIcon, FolderIcon, SystemGlyph } from "./icons";
import type { ConsoleGroup, DownloadModalState } from "./types";
import { displayRegionCode, formatCompactDate, formatCountLabel, pluralize } from "./helpers";

const SORTABLE_COLUMNS: Array<{ key: SaveSortKey; label: string; align?: "left" | "center" }> = [
  { key: "game", label: "Gamename" },
  { key: "region", label: "Region", align: "center" },
  { key: "saves", label: "Saves" },
  { key: "latest", label: "Latest" },
  { key: "total", label: "Total" },
  { key: "date", label: "Date" }
];

type MyGamesLibraryTableProps = {
  consoleGroups: ConsoleGroup[];
  expandedGroups: Record<string, boolean>;
  deletingKeys: string[];
  sortKey: SaveSortKey;
  sortDirection: SaveSortDirection;
  onToggleGroup: (groupKey: string) => void;
  onSort: (sortKey: SaveSortKey) => void;
  onOpenSaveSelector: (row: SaveRow) => void;
  onOpenCheats: (row: SaveRow) => void;
  onOpenDownload: (title: string, request: DownloadModalState["request"], profiles: SaveDownloadProfile[] | undefined) => void;
  onRequestDelete: (row: SaveRow) => void;
};

export function MyGamesLibraryTable({
  consoleGroups,
  expandedGroups,
  deletingKeys,
  sortKey,
  sortDirection,
  onToggleGroup,
  onSort,
  onOpenSaveSelector,
  onOpenCheats,
  onOpenDownload,
  onRequestDelete
}: MyGamesLibraryTableProps): JSX.Element {
  return (
    <div className="treegrid-table-wrap">
      <table className="treegrid-table" role="treegrid" aria-label="My Saves">
        <thead>
          <tr>
            {SORTABLE_COLUMNS.map((column) => (
              <th key={column.key} className={column.align === "center" ? "treegrid-table__align-center" : undefined}>
                <button className="treegrid-sort" type="button" onClick={() => onSort(column.key)} aria-label={`Sort by ${column.label}`}>
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
                    onClick={() => onToggleGroup(group.key)}
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
                              onClick={() => onOpenSaveSelector(row)}
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
                              onClick={() => onOpenCheats(row)}
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
                          <Link className="treegrid-icon-button" to={detailsHref} aria-label={`View details for ${row.gameName}`} title={`View details for ${row.gameName}`}>
                            <DetailsIcon />
                          </Link>
                        </td>
                        <td>
                          <button
                            className="treegrid-icon-button treegrid-icon-button--download"
                            type="button"
                            onClick={() => onOpenDownload(row.gameName, row.downloadRequest, row.downloadProfiles)}
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
                            onClick={() => onRequestDelete(row)}
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
  );
}
