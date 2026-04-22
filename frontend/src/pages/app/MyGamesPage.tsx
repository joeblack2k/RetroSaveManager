import { useCallback, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { deleteManySaves, deleteSave, getCurrentUser, listSaves } from "../../services/retrosaveApi";
import type { SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";

type SaveRow = {
  key: string;
  gameID: number;
  primarySaveID: string;
  gameName: string;
  coverArtUrl: string | null;
  regionCode: string;
  regionFlag: string;
  languageCodes: string[];
  systemName: string;
  systemSlug: string;
  saveIDs: string[];
  saveCount: number;
  latestSizeBytes: number;
  totalBytes: number;
  latestCreatedAt: string;
  latestVersion: number;
  downloadUrl: string;
  isPlayStationEntry: boolean;
  psLogicalKey?: string;
  sourceCardName?: string;
};

type ConsoleGroup = {
  key: string;
  name: string;
  rows: SaveRow[];
  saveCount: number;
  totalBytes: number;
};

const DEFAULT_LIMIT_BYTES = 200 * 1024 * 1024;

export function MyGamesPage(): JSX.Element {
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingKeys, setDeletingKeys] = useState<string[]>([]);

  const loader = useCallback(async () => {
    const [user, saves] = await Promise.all([getCurrentUser(), listSaves()]);
    return { user, saves };
  }, []);

  const { loading, error, data, reload } = useAsyncData(loader, []);

  const rows = useMemo<SaveRow[]>(() => {
    if (!data) {
      return [];
    }
    return buildSaveRows(data.saves);
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
      group.rows.sort((a, b) => new Date(b.latestCreatedAt).getTime() - new Date(a.latestCreatedAt).getTime());
    }
    groups.sort((a, b) => {
      if (a.name === "Other") {
        return 1;
      }
      if (b.name === "Other") {
        return -1;
      }
      return a.name.localeCompare(b.name);
    });
    return groups;
  }, [rows]);

  const totalSaveCount = useMemo(() => rows.reduce((sum, row) => sum + row.saveCount, 0), [rows]);
  const totalBytes = useMemo(() => rows.reduce((sum, row) => sum + row.totalBytes, 0), [rows]);
  const usagePercent = Math.min(100, (totalBytes / DEFAULT_LIMIT_BYTES) * 100);

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
      reload();
    } catch (err: unknown) {
      setDeleteError(err instanceof Error ? err.message : "Delete mislukt");
    } finally {
      setDeletingKeys((current) => current.filter((key) => key !== row.key));
    }
  }

  if (loading) {
    return <LoadingState label="My Saves laden..." />;
  }

  if (error) {
    return <ErrorState message={error} />;
  }

  return (
    <section className="saves-board fade-in-up">
      <header className="saves-board__header">
        <div>
          <h2>My Saves</h2>
          <p>
            {rows.length} games ({totalSaveCount} saves) · {formatBytes(totalBytes)} / {formatBytes(DEFAULT_LIMIT_BYTES)}{" "}
            <span className="saves-plan-tag">Free</span>
          </p>
          <div className="saves-board__progress" aria-label="Storage usage">
            <span style={{ width: `${usagePercent}%` }} />
          </div>
        </div>
        <div className="saves-board__actions">
          <button className="saves-toolbar-btn" type="button">
            Upload
          </button>
        </div>
      </header>
      {deleteError ? <p className="error-state">{deleteError}</p> : null}

      {consoleGroups.map((group, index) => (
        <details key={group.key} className="saves-console-group" open={index === 0}>
          <summary>
            <strong>{group.name}</strong>
            <span>
              {group.rows.length} games · {group.saveCount} saves · {formatBytes(group.totalBytes)}
            </span>
          </summary>

          <div className="saves-table-wrap">
            <table className="saves-table">
              <thead>
                <tr>
                  <th>Game</th>
                  <th>Region</th>
                  <th>Saves</th>
                  <th>Latest</th>
                  <th>Total</th>
                  <th>Rollback</th>
                  <th>Date</th>
                  <th>Details</th>
                  <th>Download</th>
                  <th>Delete</th>
                </tr>
              </thead>
              <tbody>
                {group.rows.map((row) => {
                  const isDeleting = deletingKeys.includes(row.key);
                  const detailsHref = row.psLogicalKey
                    ? `/app/saves/${encodeURIComponent(row.primarySaveID)}?psLogicalKey=${encodeURIComponent(row.psLogicalKey)}`
                    : `/app/saves/${encodeURIComponent(row.primarySaveID)}`;
                  return (
                    <tr key={row.key}>
                      <td>
                        <div className="saves-game-cell">
                          {row.coverArtUrl ? (
                            <img
                              className="saves-cover-art"
                              src={row.coverArtUrl}
                              alt=""
                              loading="lazy"
                              width={44}
                              height={44}
                            />
                          ) : null}
                          <div>
                            <strong>{row.gameName}</strong>
                            {row.sourceCardName ? <p>{row.sourceCardName}</p> : null}
                            {row.languageCodes.length > 0 ? <p>{row.languageCodes.join(", ")}</p> : null}
                          </div>
                        </div>
                      </td>
                      <td className="saves-region-cell">
                        <span className="saves-region-flag" title={row.regionCode}>{row.regionFlag}</span>
                      </td>
                      <td>{row.saveCount}</td>
                      <td>{formatBytes(row.latestSizeBytes)}</td>
                      <td>{formatBytes(row.totalBytes)}</td>
                      <td>
                        <Link className="saves-action-link" to={detailsHref}>Rollback</Link>
                      </td>
                      <td>{formatDate(row.latestCreatedAt)}</td>
                      <td>
                        <Link className="saves-action-link" to={detailsHref}>Details</Link>
                      </td>
                      <td>
                        <a className="saves-download-btn" href={row.downloadUrl}>
                          <span aria-hidden="true">⇩</span>
                          <span className="sr-only">Download {row.gameName}</span>
                        </a>
                      </td>
                      <td>
                        <button
                          className="saves-delete-btn"
                          type="button"
                          onClick={() => void handleDeleteRow(row)}
                          disabled={isDeleting}
                          aria-label={`Delete ${row.gameName} saves`}
                        >
                          {isDeleting ? "..." : "Delete"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </details>
      ))}
    </section>
  );
}

function normalizeRegionCode(regionCode: string): string {
  const normalized = regionCode.trim().toUpperCase();
  switch (normalized) {
    case "US":
    case "USA":
      return "US";
    case "EU":
    case "EUR":
      return "EU";
    case "JP":
    case "JPN":
      return "JP";
    default:
      return "UNKNOWN";
  }
}

function pickPreferredRegionCode(current: string, candidate: string): string {
  const currentNormalized = normalizeRegionCode(current);
  const candidateNormalized = normalizeRegionCode(candidate);
  if (currentNormalized === "UNKNOWN" && candidateNormalized !== "UNKNOWN") {
    return candidateNormalized;
  }
  return currentNormalized;
}

function regionToFlagEmoji(regionCode: string): string {
  switch (normalizeRegionCode(regionCode)) {
    case "US":
      return "🇺🇸";
    case "EU":
      return "🇪🇺";
    case "JP":
      return "🇯🇵";
    default:
      return "🌐";
  }
}

function cleanGameTitle(raw: string): string {
  let title = raw.trim();
  const tagPattern = /\s*\(([^)]*)\)\s*$/;
  while (true) {
    const match = tagPattern.exec(title);
    if (!match) {
      break;
    }
    const tag = (match[1] || "").trim().toLowerCase();
    if (!looksLikeMetadataTag(tag)) {
      break;
    }
    title = title.slice(0, match.index).trim();
  }
  const trailingCounterPattern = /(?:[_.-]+|\s+(?:slot|save)\s+)#?\d{1,3}$/i;
  while (trailingCounterPattern.test(title)) {
    title = title.replace(trailingCounterPattern, "").trim();
  }
  if (!title) {
    return "Unknown game";
  }
  return title;
}

function looksLikeMetadataTag(tag: string): boolean {
  if (!tag) {
    return false;
  }
  const hints = ["usa", "europe", "japan", "pal", "ntsc", "rev", "proto", "beta", "demo", "en", "ja", "fr", "de", "es", "it"];
  if (hints.some((hint) => tag.includes(hint))) {
    return true;
  }
  return /[0-9,+]/.test(tag);
}

function normalizeLanguageCode(raw: string): string | null {
  const value = raw.trim().toLowerCase();
  const map: Record<string, string> = {
    en: "EN",
    eng: "EN",
    english: "EN",
    ja: "JA",
    jp: "JA",
    jpn: "JA",
    japanese: "JA",
    fr: "FR",
    fra: "FR",
    fre: "FR",
    french: "FR",
    de: "DE",
    deu: "DE",
    ger: "DE",
    german: "DE",
    es: "ES",
    spa: "ES",
    spanish: "ES",
    it: "IT",
    ita: "IT",
    italian: "IT",
    pt: "PT",
    por: "PT",
    portuguese: "PT",
    nl: "NL",
    dut: "NL",
    nld: "NL",
    dutch: "NL"
  };
  return map[value] ?? null;
}

function extractLanguageCodes(raw: string): string[] {
  const normalized = raw.toLowerCase().replace(/[,+/_-]/g, " ");
  const parts = normalized.split(/\s+/).map((item) => item.trim()).filter((item) => item !== "");
  return mergeLanguageCodes(parts);
}

function mergeLanguageCodes(...sources: Array<string[] | undefined>): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const source of sources) {
    if (!source) {
      continue;
    }
    for (const item of source) {
      const normalized = normalizeLanguageCode(item);
      if (!normalized || seen.has(normalized)) {
        continue;
      }
      seen.add(normalized);
      out.push(normalized);
    }
  }
  return out;
}

export function detectConsoleForSave(save: SaveSummary): { slug: string; name: string } {
  const knownSystem = save.game.system?.name?.trim() || "";
  const knownSlug = save.game.system?.slug?.trim().toLowerCase() || "";
  const summarySlug = (save.systemSlug || "").trim().toLowerCase();

  if (summarySlug !== "" && !isUnknownSystemLabel(summarySlug)) {
    return normalizeConsoleLabel(summarySlug, knownSystem || summarySlug);
  }
  if (knownSlug !== "" && !isUnknownSystemLabel(knownSlug)) {
    return normalizeConsoleLabel(knownSlug, knownSystem);
  }
  if (knownSystem !== "" && !isUnknownSystemLabel(knownSystem)) {
    return normalizeConsoleLabel(knownSystem, knownSystem);
  }
  return { slug: "other", name: "Other" };
}

export function normalizeConsoleLabel(slug: string, name: string): { slug: string; name: string } {
  const normalizedSlug = normalizeConsoleKey(slug);
  const normalizedName = normalizeConsoleKey(name);
  const aliases: Record<string, { slug: string; name: string }> = {
    arcade: { slug: "arcade", name: "Arcade" },
    "game-gear": { slug: "game-gear", name: "Game Gear" },
    gameboy: { slug: "gameboy", name: "Nintendo Game Boy" },
    "nintendo-game-boy": { slug: "gameboy", name: "Nintendo Game Boy" },
    gba: { slug: "gba", name: "Game Boy Advance" },
    "game-boy-advance": { slug: "gba", name: "Game Boy Advance" },
    genesis: { slug: "genesis", name: "Sega Genesis" },
    "sega-genesis-mega-drive": { slug: "genesis", name: "Sega Genesis" },
    "master-system": { slug: "master-system", name: "Master System" },
    "n64": { slug: "n64", name: "Nintendo 64" },
    "nintendo-64": { slug: "n64", name: "Nintendo 64" },
    nds: { slug: "nds", name: "Nintendo DS" },
    "nintendo-ds": { slug: "nds", name: "Nintendo DS" },
    neogeo: { slug: "neogeo", name: "Neo Geo" },
    nes: { slug: "nes", name: "Nintendo Entertainment System" },
    "nintendo-entertainment-system": { slug: "nes", name: "Nintendo Entertainment System" },
    psx: { slug: "psx", name: "PlayStation" },
    playstation: { slug: "psx", name: "PlayStation" },
    ps2: { slug: "ps2", name: "PlayStation 2" },
    "playstation-2": { slug: "ps2", name: "PlayStation 2" },
    psp: { slug: "psp", name: "PlayStation Portable" },
    psvita: { slug: "psvita", name: "PlayStation Vita" },
    snes: { slug: "snes", name: "Super Nintendo" },
    "super-nintendo": { slug: "snes", name: "Super Nintendo" },
    "nintendo-super-nintendo-entertainment-system": { slug: "snes", name: "Super Nintendo" }
  };

  if (normalizedSlug && aliases[normalizedSlug]) {
    return aliases[normalizedSlug];
  }
  if (normalizedName && aliases[normalizedName]) {
    return aliases[normalizedName];
  }

  const cleaned = name.trim();
  if (cleaned === "" || isUnknownSystemLabel(cleaned)) {
    return { slug: "other", name: "Other" };
  }
  return {
    slug: slug.trim() !== "" ? slug.trim().toLowerCase() : cleaned.toLowerCase().replace(/\s+/g, "-"),
    name: cleaned
  };
}

function isUnknownSystemLabel(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return normalized === "" || normalized === "unknown" || normalized === "unknown-system" || normalized === "unknown system";
}

function normalizeConsoleKey(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

function buildSaveRows(saves: SaveSummary[]): SaveRow[] {
  const grouped = new Map<string, SaveRow>();

  for (const save of saves) {
    for (const row of expandSaveRows(save)) {
      const existing = grouped.get(row.key);
      if (!existing) {
        grouped.set(row.key, row);
        continue;
      }

      existing.saveCount = Math.max(existing.saveCount, row.saveCount);
      existing.latestSizeBytes = Math.max(existing.latestSizeBytes, row.latestSizeBytes);
      existing.totalBytes = Math.max(existing.totalBytes, row.totalBytes);
      existing.latestVersion = Math.max(existing.latestVersion, row.latestVersion);
      existing.regionCode = pickPreferredRegionCode(existing.regionCode, row.regionCode);
      existing.regionFlag = regionToFlagEmoji(existing.regionCode);
      existing.languageCodes = mergeLanguageCodes(existing.languageCodes, row.languageCodes);
      existing.coverArtUrl = existing.coverArtUrl || row.coverArtUrl;

      if (new Date(row.latestCreatedAt).getTime() > new Date(existing.latestCreatedAt).getTime()) {
        existing.latestCreatedAt = row.latestCreatedAt;
        existing.primarySaveID = row.primarySaveID;
        existing.downloadUrl = row.downloadUrl;
        existing.sourceCardName = row.sourceCardName;
      }

      for (const saveID of row.saveIDs) {
        if (!existing.saveIDs.includes(saveID)) {
          existing.saveIDs.push(saveID);
        }
      }
    }
  }

  const rows = [...grouped.values()];
  rows.sort((a, b) => new Date(b.latestCreatedAt).getTime() - new Date(a.latestCreatedAt).getTime());
  return rows;
}

function expandSaveRows(save: SaveSummary): SaveRow[] {
  const systemInfo = detectConsoleForSave(save);
  const logicalKey = (save.logicalKey || "").trim();
  if ((systemInfo.slug === "psx" || systemInfo.slug === "ps2")) {
    if (logicalKey !== "") {
      return [buildStandardSaveRow(save, systemInfo)];
    }
    return [];
  }
  return [buildStandardSaveRow(save, systemInfo)];
}

function buildStandardSaveRow(save: SaveSummary, systemInfo: { slug: string; name: string }): SaveRow {
  const rawName = save.displayTitle?.trim() || save.game.displayTitle?.trim() || save.game.name?.trim() || "Unknown game";
  const gameName = cleanGameTitle(rawName);
  const regionCode = normalizeRegionCode((save.regionCode || save.game.regionCode || "UNKNOWN").toString());
  const createdAt = save.createdAt || new Date(0).toISOString();
  const saveCount = save.saveCount && save.saveCount > 0 ? save.saveCount : 1;
  const latestSizeBytes = save.latestSizeBytes && save.latestSizeBytes > 0 ? save.latestSizeBytes : save.fileSize;
  const totalBytes = save.totalSizeBytes && save.totalSizeBytes > 0 ? save.totalSizeBytes : save.fileSize;
  const latestVersion = save.latestVersion && save.latestVersion > 0 ? save.latestVersion : save.version;
  const logicalKey = (save.logicalKey || "").trim();

  return {
    key: logicalKey !== "" ? `${systemInfo.slug}:${logicalKey}:${regionCode}` : `${systemInfo.slug}:${save.game.id}:${gameName}`,
    gameID: save.game.id,
    primarySaveID: save.id,
    gameName,
    coverArtUrl: save.coverArtUrl || save.game.coverArtUrl || save.game.boxartThumb || save.game.boxart || null,
    regionCode,
    regionFlag: regionToFlagEmoji(regionCode),
    languageCodes: mergeLanguageCodes(save.languageCodes, save.game.languageCodes, extractLanguageCodes(rawName)),
    systemName: systemInfo.name,
    systemSlug: systemInfo.slug,
    saveIDs: [save.id],
    saveCount,
    latestSizeBytes,
    totalBytes,
    latestCreatedAt: createdAt,
    latestVersion,
    downloadUrl: apiDownloadURL(
      logicalKey !== ""
        ? `/saves/download?id=${encodeURIComponent(save.id)}&psLogicalKey=${encodeURIComponent(logicalKey)}`
        : `/saves/download?id=${encodeURIComponent(save.id)}`
    ),
    isPlayStationEntry: logicalKey !== "",
    psLogicalKey: logicalKey !== "" ? logicalKey : undefined
  };
}
