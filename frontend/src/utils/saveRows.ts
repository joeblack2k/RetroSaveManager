import { apiDownloadURL } from "../services/apiClient";
import type { SaveSummary } from "../services/types";

export type SaveRow = {
  key: string;
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
  psLogicalKey?: string;
};

export type SaveSortKey = "game" | "region" | "saves" | "latest" | "total" | "date";
export type SaveSortDirection = "asc" | "desc";

export function buildSaveRows(saves: SaveSummary[]): SaveRow[] {
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

export function sortSaveRows(rows: SaveRow[], sortKey: SaveSortKey, direction: SaveSortDirection): SaveRow[] {
  const factor = direction === "asc" ? 1 : -1;
  return [...rows].sort((left, right) => {
    const delta = compareSaveRows(left, right, sortKey);
    if (delta !== 0) {
      return delta * factor;
    }
    return left.gameName.localeCompare(right.gameName);
  });
}

export function buildSaveDetailsHref(save: { primarySaveID: string; psLogicalKey?: string }): string {
  const base = `/app/saves/${encodeURIComponent(save.primarySaveID)}`;
  if (!save.psLogicalKey) {
    return base;
  }
  return `${base}?psLogicalKey=${encodeURIComponent(save.psLogicalKey)}`;
}

export function systemBadgeForSlug(systemSlug: string): { label: string; title: string } {
  const normalized = normalizeConsoleKey(systemSlug);
  const map: Record<string, { label: string; title: string }> = {
    arcade: { label: "AR", title: "Arcade" },
    gba: { label: "GBA", title: "Game Boy Advance" },
    gameboy: { label: "GB", title: "Game Boy" },
    genesis: { label: "GEN", title: "Genesis" },
    "master-system": { label: "SMS", title: "Master System" },
    n64: { label: "N64", title: "Nintendo 64" },
    nds: { label: "DS", title: "Nintendo DS" },
    neogeo: { label: "NG", title: "Neo Geo" },
    nes: { label: "NES", title: "Nintendo Entertainment System" },
    psx: { label: "PS", title: "PlayStation" },
    ps2: { label: "PS2", title: "PlayStation 2" },
    psp: { label: "PSP", title: "PlayStation Portable" },
    psvita: { label: "VITA", title: "PlayStation Vita" },
    snes: { label: "SNES", title: "Super Nintendo" },
    other: { label: "?", title: "Other" }
  };

  return map[normalized] ?? map.other;
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
    n64: { slug: "n64", name: "Nintendo 64" },
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

export function normalizeRegionCode(regionCode: string): string {
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

export function regionToFlagEmoji(regionCode: string): string {
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

function compareSaveRows(left: SaveRow, right: SaveRow, sortKey: SaveSortKey): number {
  switch (sortKey) {
    case "game":
      return left.gameName.localeCompare(right.gameName);
    case "region":
      return left.regionCode.localeCompare(right.regionCode);
    case "saves":
      return left.saveCount - right.saveCount;
    case "latest":
      return left.latestSizeBytes - right.latestSizeBytes;
    case "total":
      return left.totalBytes - right.totalBytes;
    case "date":
      return new Date(left.latestCreatedAt).getTime() - new Date(right.latestCreatedAt).getTime();
    default:
      return 0;
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
  return title || "Unknown game";
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

function isUnknownSystemLabel(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return normalized === "" || normalized === "unknown" || normalized === "unknown-system" || normalized === "unknown system";
}

function normalizeConsoleKey(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

function expandSaveRows(save: SaveSummary): SaveRow[] {
  const systemInfo = detectConsoleForSave(save);
  const logicalKey = (save.logicalKey || "").trim();
  if (systemInfo.slug === "psx" || systemInfo.slug === "ps2") {
    if (logicalKey === "") {
      return [];
    }
    return [buildStandardSaveRow(save, systemInfo)];
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
    psLogicalKey: logicalKey !== "" ? logicalKey : undefined
  };
}
