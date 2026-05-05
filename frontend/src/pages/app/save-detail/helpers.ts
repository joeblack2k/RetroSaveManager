import type { MemoryCardEntry, SaveCheatEditorState, SaveCheatField, SaveSummary } from "../../../services/types";
import type { DetailMetric } from "./types";

export function buildCheatFactRows(cheats: SaveCheatEditorState | null): DetailMetric[] {
  if (!cheats?.supported) {
    return [];
  }
  const { values, slotLabel } = selectCheatValues(cheats);
  const rows: DetailMetric[] = [];
  if (slotLabel) {
    rows.push({ label: "Save file", value: slotLabel });
  }

  const fields = (cheats.sections ?? []).flatMap((section) => section.fields ?? []);
  const seenKeys = new Set<string>();
  for (const field of fields) {
    const key = field.ref || field.id;
    if (!key || seenKeys.has(key) || !Object.prototype.hasOwnProperty.call(values, key)) {
      continue;
    }
    const formatted = formatCheatValue(field, values[key]);
    if (!formatted) {
      continue;
    }
    seenKeys.add(key);
    rows.push({ label: field.label || humanizeDetailLabel(key), value: formatted });
    if (rows.length >= 18) {
      return rows;
    }
  }

  for (const [key, value] of Object.entries(values)) {
    if (seenKeys.has(key) || rows.length >= 18) {
      continue;
    }
    const formatted = formatBasicDetailValue(value);
    if (!formatted) {
      continue;
    }
    rows.push({ label: humanizeDetailLabel(key), value: formatted });
  }

  return rows;
}

function selectCheatValues(cheats: SaveCheatEditorState): { values: Record<string, unknown>; slotLabel: string } {
  if (cheats.values && Object.keys(cheats.values).length > 0) {
    return { values: cheats.values, slotLabel: "" };
  }
  const slotValues = cheats.slotValues ?? {};
  const selectorOptions = cheats.selector?.options ?? [];
  for (const option of selectorOptions) {
    const values = slotValues[option.id];
    if (values && Object.keys(values).length > 0) {
      return { values, slotLabel: option.label || option.id };
    }
  }
  const firstSlot = Object.keys(slotValues)[0];
  if (firstSlot) {
    return { values: slotValues[firstSlot] ?? {}, slotLabel: firstSlot };
  }
  return { values: {}, slotLabel: "" };
}

export function mergeDetailRows(primary: DetailMetric[], secondary: DetailMetric[]): DetailMetric[] {
  const rows: DetailMetric[] = [];
  const seen = new Set<string>();
  for (const row of [...primary, ...secondary]) {
    const key = row.label.trim().toLowerCase();
    if (!row.value || seen.has(key)) {
      continue;
    }
    seen.add(key);
    rows.push(row);
  }
  return rows;
}

function formatCheatValue(field: SaveCheatField, value: unknown): string {
  if (field.type === "boolean" && typeof value === "boolean") {
    return value ? "Enabled" : "Disabled";
  }
  if (field.type === "enum" && typeof value === "string") {
    return field.options?.find((option) => option.id === value)?.label || humanizeDetailLabel(value);
  }
  if (field.type === "bitmask" && Array.isArray(value)) {
    const labels = value
      .map((item) => String(item))
      .map((item) => field.bits?.find((bit) => bit.id === item)?.label || item)
      .filter(Boolean);
    return labels.join(", ");
  }
  return formatBasicDetailValue(value);
}

function formatBasicDetailValue(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "";
  }
  if (typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? String(value) : "";
  }
  if (typeof value === "string") {
    return value.trim();
  }
  if (Array.isArray(value)) {
    return value.map((item) => formatBasicDetailValue(item)).filter(Boolean).join(", ");
  }
  return "";
}

function humanizeDetailLabel(value: string): string {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim()
    .replace(/\s+/g, " ")
    .replace(/^./, (char) => char.toUpperCase());
}

export function uniqueMemoryCardEntries(entries: MemoryCardEntry[] | undefined): MemoryCardEntry[] {
  if (!entries || entries.length === 0) {
    return [];
  }
  const out: MemoryCardEntry[] = [];
  const seen = new Set<string>();
  for (const entry of entries) {
    const logicalKey = (entry.logicalKey || "").trim();
    const identity = logicalKey || `${entry.productCode || entry.directoryName || entry.title}:${entry.slot}:${entry.blocks}`;
    if (seen.has(identity)) {
      continue;
    }
    seen.add(identity);
    out.push(entry);
  }
  out.sort((a, b) => {
    const slotDelta = (a.slot || 0) - (b.slot || 0);
    if (slotDelta !== 0) {
      return slotDelta;
    }
    return a.title.localeCompare(b.title);
  });
  return out;
}

export function normalizeSystemSlug(value: string): string {
  return value.trim().toLowerCase();
}

export function buildSummaryFromVersions(versions: SaveSummary[]): {
  saveCount: number;
  totalSizeBytes: number;
  latestVersion: number;
  latestCreatedAt: string;
  languageCodes: string[];
} {
  if (versions.length === 0) {
    return {
      saveCount: 0,
      totalSizeBytes: 0,
      latestVersion: 0,
      latestCreatedAt: "",
      languageCodes: []
    };
  }
  const totalSizeBytes = versions.reduce((sum, item) => sum + item.fileSize, 0);
  return {
    saveCount: versions.length,
    totalSizeBytes,
    latestVersion: versions[0].version,
    latestCreatedAt: versions[0].createdAt,
    languageCodes: mergeLanguageCodes(...versions.map((item) => item.languageCodes))
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

export function mergeLanguageCodes(...sources: Array<string[] | undefined>): string[] {
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
