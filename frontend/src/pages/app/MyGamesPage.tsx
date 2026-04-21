import { useCallback, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { deleteManySaves, getCurrentUser, listSaves } from "../../services/retrosaveApi";
import type { SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";

type SaveRow = {
  key: string;
  gameID: number;
  primarySaveID: string;
  gameName: string;
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
    const grouped = new Map<string, SaveRow & { saveIDs: string[] }>();

    for (const save of data.saves) {
      const systemInfo = detectConsoleForSave(save);
      const systemSlug = systemInfo.slug;
      const systemName = systemInfo.name;
      const rawName = save.displayTitle?.trim() || save.game.displayTitle?.trim() || save.game.name?.trim() || "Unknown game";
      const gameName = cleanGameTitle(rawName);
      const regionCode = normalizeRegionCode((save.regionCode || save.game.regionCode || "UNKNOWN").toString());
      const languageCodes = mergeLanguageCodes(save.languageCodes, save.game.languageCodes, extractLanguageCodes(rawName));
      const key = `${systemSlug}:${save.game.id}:${gameName}`;
      const createdAt = save.createdAt || new Date(0).toISOString();

      const existing = grouped.get(key);
      if (!existing) {
        const saveCount = save.saveCount && save.saveCount > 0 ? save.saveCount : 1;
        const latestSizeBytes = save.latestSizeBytes && save.latestSizeBytes > 0 ? save.latestSizeBytes : save.fileSize;
        const totalBytes = save.totalSizeBytes && save.totalSizeBytes > 0 ? save.totalSizeBytes : save.fileSize;
        const latestVersion = save.latestVersion && save.latestVersion > 0 ? save.latestVersion : save.version;

        grouped.set(key, {
          key,
          gameID: save.game.id,
          primarySaveID: save.id,
          gameName,
          regionCode,
          regionFlag: regionToFlagEmoji(regionCode),
          languageCodes,
          systemName,
          systemSlug,
          saveCount,
          latestSizeBytes,
          totalBytes,
          latestCreatedAt: createdAt,
          latestVersion,
          saveIDs: [save.id],
          downloadUrl: ""
        });
        continue;
      }

      existing.saveCount = Math.max(existing.saveCount, save.saveCount && save.saveCount > 0 ? save.saveCount : existing.saveCount + 1);
      existing.latestSizeBytes = Math.max(existing.latestSizeBytes, save.latestSizeBytes && save.latestSizeBytes > 0 ? save.latestSizeBytes : save.fileSize);
      existing.totalBytes = Math.max(existing.totalBytes, save.totalSizeBytes && save.totalSizeBytes > 0 ? save.totalSizeBytes : existing.totalBytes + save.fileSize);
      existing.latestVersion = Math.max(existing.latestVersion, save.latestVersion && save.latestVersion > 0 ? save.latestVersion : save.version);
      existing.regionCode = pickPreferredRegionCode(existing.regionCode, regionCode);
      existing.regionFlag = regionToFlagEmoji(existing.regionCode);
      existing.languageCodes = mergeLanguageCodes(existing.languageCodes, languageCodes);

      if (new Date(createdAt).getTime() > new Date(existing.latestCreatedAt).getTime()) {
        existing.latestCreatedAt = createdAt;
        existing.primarySaveID = save.id;
      }
      existing.saveIDs.push(save.id);
    }

    const list = [...grouped.values()].map((row) => ({
      key: row.key,
      gameID: row.gameID,
      primarySaveID: row.primarySaveID,
      gameName: row.gameName,
      regionCode: row.regionCode,
      regionFlag: row.regionFlag,
      languageCodes: row.languageCodes,
      systemName: row.systemName,
      systemSlug: row.systemSlug,
      saveIDs: row.saveIDs,
      saveCount: row.saveCount,
      latestSizeBytes: row.latestSizeBytes,
      totalBytes: row.totalBytes,
      latestCreatedAt: row.latestCreatedAt,
      latestVersion: row.latestVersion,
      downloadUrl: row.saveIDs.length === 1
        ? apiDownloadURL(`/saves/download?id=${encodeURIComponent(row.saveIDs[0])}`)
        : apiDownloadURL(`/saves/download_many?ids=${encodeURIComponent(row.saveIDs.join(","))}`)
    }));

    list.sort((a, b) => new Date(b.latestCreatedAt).getTime() - new Date(a.latestCreatedAt).getTime());
    return list;
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
      await deleteManySaves(row.saveIDs);
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
                  const detailsHref = `/app/saves/${encodeURIComponent(row.primarySaveID)}`;
                  return (
                    <tr key={row.key}>
                      <td>
                        <div className="saves-game-cell">
                          <div>
                            <strong>{row.gameName}</strong>
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

function detectConsoleForSave(save: SaveSummary): { slug: string; name: string } {
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
    return normalizeConsoleLabel(knownSlug, knownSystem);
  }

  const filename = save.filename.toLowerCase();
  const ext = filename.includes(".") ? filename.split(".").pop() || "" : "";
  const title = (save.displayTitle || save.game.displayTitle || save.game.name || "").toLowerCase();
  const format = (save.format || "").toLowerCase();

  if (["mcr", "mcd", "gme", "mc"].includes(ext) || format.includes("mcr") || title.includes("memory card")) {
    return { slug: "psx", name: "PlayStation" };
  }
  if (["dsv"].includes(ext)) {
    return { slug: "nds", name: "Nintendo DS" };
  }
  if (["eep", "fla", "mpk", "sra"].includes(ext)) {
    return { slug: "n64", name: "Nintendo 64" };
  }
  if (["ps2"].includes(ext)) {
    return { slug: "ps2", name: "PlayStation 2" };
  }
  if (["nv", "nvram", "hi", "eeprom"].includes(ext) || isArcadeHint(`${filename} ${title}`)) {
    return { slug: "arcade", name: "Arcade" };
  }
  if (["srm", "smc", "sfc"].includes(ext)) {
    return { slug: "snes", name: "Super Nintendo" };
  }
  if (["ram"].includes(ext) && isArcadeHint(`${filename} ${title}`)) {
    return { slug: "arcade", name: "Arcade" };
  }
  return { slug: "other", name: "Other" };
}

function normalizeConsoleLabel(slug: string, name: string): { slug: string; name: string } {
  const combined = `${slug} ${name}`.toLowerCase();
  if (combined.includes("arcade") || combined.includes("mame") || combined.includes("fbneo") || combined.includes("finalburn")) {
    return { slug: "arcade", name: "Arcade" };
  }
  if (combined.includes("nds") || combined.includes("nintendo ds")) {
    return { slug: "nds", name: "Nintendo DS" };
  }
  if (combined.includes("nes") || combined.includes("famicom")) {
    return { slug: "nes", name: "Nintendo Entertainment System" };
  }
  if (combined.includes("snes") || combined.includes("super nintendo")) {
    return { slug: "snes", name: "Super Nintendo" };
  }
  if (combined.includes("n64") || combined.includes("nintendo 64")) {
    return { slug: "n64", name: "Nintendo 64" };
  }
  if (combined.includes("neogeo") || combined.includes("neo geo")) {
    return { slug: "neogeo", name: "Neo Geo" };
  }
  if (combined.includes("ps2") || combined.includes("playstation 2")) {
    return { slug: "ps2", name: "PlayStation 2" };
  }
  if (combined.includes("psp") || combined.includes("playstation portable")) {
    return { slug: "psp", name: "PlayStation Portable" };
  }
  if (combined.includes("psvita") || combined.includes("ps vita")) {
    return { slug: "psvita", name: "PlayStation Vita" };
  }
  if (combined.includes("playstation") || combined.includes("psx") || combined.includes("ps1")) {
    return { slug: "psx", name: "PlayStation" };
  }
  if (combined.includes("master system") || combined.includes("sms")) {
    return { slug: "master-system", name: "Master System" };
  }
  if (combined.includes("game gear")) {
    return { slug: "game-gear", name: "Game Gear" };
  }
  if (combined.includes("game boy") || combined.includes("gameboy")) {
    return { slug: "gameboy", name: "Nintendo Game Boy" };
  }
  if (combined.includes("gba")) {
    return { slug: "gba", name: "Game Boy Advance" };
  }
  if (combined.includes("genesis") || combined.includes("mega drive")) {
    return { slug: "genesis", name: "Sega Genesis" };
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

function isArcadeHint(raw: string): boolean {
  const value = raw.toLowerCase();
  return ["arcade", "mame", "fbneo", "finalburn", "model2", "naomi", "daytona", "ghost house"].some((hint) => value.includes(hint));
}

function isUnknownSystemLabel(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return normalized === "" || normalized === "unknown" || normalized === "unknown-system" || normalized === "unknown system";
}
