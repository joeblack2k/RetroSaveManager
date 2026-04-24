import type { SaveInspection, SaveSummary } from "../services/types";
import { formatBytes } from "./format";

export type SaveInsightRow = {
  label: string;
  value: string;
  kind?: "gameplay" | "technical";
};

export type SaveInsightModel = {
  title: string;
  subtitle: string;
  parserLevel: string;
  parserId: string;
  rows: SaveInsightRow[];
  warnings: string[];
  evidence: string[];
};

type GameplayField = {
  label: string;
  keys: string[];
};

const gameplayFields: GameplayField[] = [
  { label: "Lives", keys: ["lives", "lifeCount", "currentLives", "playerLives"] },
  { label: "Current map", keys: ["map", "currentMap", "mapName", "location", "currentLocation"] },
  { label: "World", keys: ["world", "currentWorld"] },
  { label: "Level", keys: ["level", "currentLevel", "stage", "currentStage"] },
  { label: "Zone", keys: ["zone", "currentZone"] },
  { label: "Act", keys: ["act", "currentAct"] },
  { label: "Progress", keys: ["progress", "completion", "completionPercent", "percentComplete"] },
  { label: "Stars", keys: ["stars", "starCount", "collectedStars"] },
  { label: "Coins", keys: ["coins", "coinCount"] },
  { label: "Hearts", keys: ["hearts", "heartContainers"] },
  { label: "Keys", keys: ["keys", "keyCount"] }
];

export function buildSaveInsight(save: Pick<SaveSummary, "inspection" | "fileSize"> | null | undefined): SaveInsightModel | null {
  const inspection = save?.inspection;
  if (!inspection) {
    return null;
  }

  const fields = asRecord(inspection.semanticFields);
  const rows: SaveInsightRow[] = [];
  const seenLabels = new Set<string>();
  const addRow = (label: string, value: unknown, kind: SaveInsightRow["kind"] = "technical"): void => {
    const formatted = formatInsightValue(value);
    if (!formatted || seenLabels.has(label)) {
      return;
    }
    seenLabels.add(label);
    rows.push({ label, value: formatted, kind });
  };

  for (const field of gameplayFields) {
    const value = firstFieldValue(fields, field.keys);
    addRow(field.label, value, "gameplay");
  }

  addRow("Validated game", inspection.validatedGameTitle);
  addRow("Parser", inspection.parserId);
  addRow("Trust", humanizeTrustLevel(inspection.trustLevel));
  addRow("Checksum", typeof inspection.checksumValid === "boolean" ? (inspection.checksumValid ? "Valid" : "Invalid") : undefined);
  addRow("Active save slots", inspection.activeSlotIndexes);
  addRow("Slot count", inspection.slotCount);
  addRow("Raw save kind", fields.rawSaveKind);
  addRow("Media type", fields.mediaType);
  addRow("ROM link", firstFieldValue(fields, ["romLinked", "romSha1Present"]) === true ? "Present" : undefined);
  addRow("Blank check", fields.blankCheck);
  addRow("File extension", fields.extension);
  addRow("Game profile", humanizeVariant(fields.variant));
  addRow("Valid copies", fields.validCopies);
  addRow("Mirrored copies", typeof fields.identicalCopies === "boolean" ? (fields.identicalCopies ? "Identical" : "Different") : undefined);
  addRow("Primary slots", fields.validPrimarySlots);
  addRow("Backup slots", fields.validBackupSlots);
  addRow("Controller Pak entries", fields.entryCount);
  addRow("Signatures", fields.signatures);
  addRow("Signature count", fields.signatureCount);
  addRow("Default volume", fields.defaultVolume);
  addRow("Container format", fields.format);
  addRow("Payload size", formatBytes(inspection.payloadSizeBytes ?? save?.fileSize), "technical");
  addRow("Non-zero bytes", fields.nonZeroBytes);
  addRow("Non-FF bytes", fields.nonFFBytes);

  return {
    title: titleForInspection(inspection),
    subtitle: subtitleForInspection(inspection),
    parserLevel: humanizeParserLevel(inspection.parserLevel),
    parserId: inspection.parserId || "unknown",
    rows,
    warnings: stringArray(inspection.warnings).slice(0, 4),
    evidence: stringArray(inspection.evidence).slice(0, 12)
  };
}

function titleForInspection(inspection: SaveInspection): string {
  if (isRawMediaInspection(inspection)) {
    return "Raw cartridge save verified";
  }
  const parserLevel = inspection.parserLevel;
  switch ((parserLevel || "").toLowerCase()) {
    case "semantic":
      return "Gameplay decoder active";
    case "structural":
      return "Save structure verified";
    case "container":
      return "Raw save media verified";
    default:
      return "Save metadata";
  }
}

function subtitleForInspection(inspection: SaveInspection): string {
  if (isRawMediaInspection(inspection)) {
    return "NES, Game Boy, GBA, SNES, and Sega raw saves are validated as real cartridge save media; gameplay stats need a per-game decoder.";
  }
  const parserLevel = inspection.parserLevel;
  switch ((parserLevel || "").toLowerCase()) {
    case "semantic":
      return "Parser-backed gameplay facts are available for this save.";
    case "structural":
      return "We can verify the save structure; exact lives or map need a game-specific semantic decoder.";
    case "container":
      return "We can verify the save media and protect it, but gameplay stats are not decoded yet.";
    default:
      return "Only verified backend metadata is shown here.";
  }
}

function isRawMediaInspection(inspection: SaveInspection): boolean {
  const parserId = (inspection.parserId || "").toLowerCase();
  return (inspection.parserLevel || "").toLowerCase() === "container" && (
    parserId.endsWith("-raw-sram") ||
    parserId === "gba-raw-backup" ||
    parserId === "n64-save-media"
  );
}

function firstFieldValue(fields: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    if (fields[key] !== undefined && fields[key] !== null && fields[key] !== "") {
      return fields[key];
    }
  }
  return undefined;
}

function formatInsightValue(value: unknown): string {
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
    const formatted = value.map((item) => formatInsightValue(item)).filter((item) => item.length > 0);
    return formatted.join(", ");
  }
  return JSON.stringify(value);
}

function humanizeParserLevel(value: string | undefined): string {
  switch ((value || "").toLowerCase()) {
    case "semantic":
      return "Semantic";
    case "structural":
      return "Structural";
    case "container":
      return "Container";
    case "none":
      return "None";
    default:
      return value || "Unknown";
  }
}

function humanizeTrustLevel(value: string | undefined): string {
  switch ((value || "").toLowerCase()) {
    case "semantic-validated":
      return "Semantic validated";
    case "game-validated":
      return "Game validated";
    case "rom-media-verified":
      return "ROM + media verified";
    case "media-only":
      return "Media only";
    default:
      return value || "";
  }
}

function humanizeVariant(value: unknown): string {
  if (typeof value !== "string") {
    return formatInsightValue(value);
  }
  const normalized = value.trim();
  if (!normalized) {
    return "";
  }
  const upperAliases: Record<string, string> = {
    oot: "Ocarina of Time",
    dkc1: "Donkey Kong Country",
    dkc3: "Donkey Kong Country 3"
  };
  const alias = upperAliases[normalized.toLowerCase()];
  if (alias) {
    return alias;
  }
  return normalized
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function asRecord(value: SaveInspection["semanticFields"]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  return value;
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
}
