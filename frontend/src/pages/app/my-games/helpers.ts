import type { SaveCheatEditorState, SaveCheatField } from "../../../services/types";
import { formatDate } from "../../../utils/format";
import type { SaveSortDirection, SaveSortKey } from "../../../utils/saveRows";

export function defaultCheatSlotId(data: SaveCheatEditorState | null): string {
  return data?.selector?.options?.[0]?.id ?? "";
}

export function sanitizeCheatDraftValue(field: SaveCheatField, value: unknown): unknown {
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

export function pluralize(value: number, singular: string, plural: string): string {
  return value === 1 ? singular : plural;
}

export function formatCountLabel(value: number, singular: string, plural: string): string {
  return `${value} ${pluralize(value, singular, plural)}`;
}

export function displayRegionCode(regionCode: string): string {
  return regionCode === "UNKNOWN" ? "Other" : regionCode;
}

export function formatCompactDate(iso: string): string {
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

export function defaultDirectionFor(sortKey: SaveSortKey): SaveSortDirection {
  switch (sortKey) {
    case "game":
    case "region":
      return "asc";
    default:
      return "desc";
  }
}

const RUNTIME_PROFILE_OPTIONS: Array<{ system: string; value: string; label: string }> = [
  { system: "n64", value: "n64/mister", label: "MiSTer N64" },
  { system: "n64", value: "n64/retroarch", label: "RetroArch N64" },
  { system: "n64", value: "n64/project64", label: "Project64" },
  { system: "n64", value: "n64/mupen-family", label: "Mupen/RMG" },
  { system: "n64", value: "n64/everdrive", label: "EverDrive" },
  { system: "snes", value: "snes/snes9x", label: "Snes9x" },
  { system: "snes", value: "snes/bsnes", label: "bsnes" },
  { system: "snes", value: "snes/retroarch-snes9x", label: "RetroArch Snes9x" },
  { system: "snes", value: "snes/mesen2", label: "Mesen 2" },
  { system: "snes", value: "snes/higan", label: "higan" },
  { system: "nes", value: "nes/mesen2", label: "Mesen 2" },
  { system: "nes", value: "nes/fceux", label: "FCEUX" },
  { system: "nes", value: "nes/nestopia-ue", label: "Nestopia UE" },
  { system: "nes", value: "nes/punes", label: "puNES" },
  { system: "nes", value: "nes/retroarch-nestopia", label: "RetroArch Nestopia" },
  { system: "nes", value: "nes/retroarch-fceumm", label: "RetroArch FCEUmm" },
  { system: "gba", value: "gba/mgba", label: "mGBA" },
  { system: "gba", value: "gba/vba-m", label: "VBA-M" },
  { system: "gba", value: "gba/nocashgba", label: "No$GBA" },
  { system: "gba", value: "gba/skyemu", label: "SkyEmu" },
  { system: "gba", value: "gba/retroarch-mgba", label: "RetroArch mGBA" },
  { system: "gba", value: "gba/retroarch-vbam", label: "RetroArch VBA-M" },
  { system: "genesis", value: "genesis/genesis-plus-gx", label: "Genesis Plus GX" },
  { system: "genesis", value: "genesis/picodrive", label: "PicoDrive" },
  { system: "genesis", value: "genesis/blastem", label: "BlastEm" },
  { system: "genesis", value: "genesis/retroarch-genesis-plus-gx", label: "RetroArch Genesis Plus GX" },
  { system: "genesis", value: "genesis/retroarch-picodrive", label: "RetroArch PicoDrive" },
  { system: "sega-cd", value: "sega-cd/genesis-plus-gx", label: "Genesis Plus GX" },
  { system: "sega-cd", value: "sega-cd/picodrive", label: "PicoDrive" },
  { system: "sega-cd", value: "sega-cd/retroarch-genesis-plus-gx", label: "RetroArch Genesis Plus GX" },
  { system: "sega-cd", value: "sega-cd/retroarch-picodrive", label: "RetroArch PicoDrive" },
  { system: "sega-32x", value: "sega-32x/picodrive", label: "PicoDrive" },
  { system: "sega-32x", value: "sega-32x/genesis-plus-gx", label: "Genesis Plus GX" },
  { system: "sega-32x", value: "sega-32x/retroarch-picodrive", label: "RetroArch PicoDrive" },
  { system: "master-system", value: "sms/genesis-plus-gx", label: "Genesis Plus GX SMS" },
  { system: "master-system", value: "sms/emulicious", label: "Emulicious" },
  { system: "master-system", value: "sms/meka", label: "MEKA" },
  { system: "master-system", value: "sms/retroarch-gearsystem", label: "RetroArch Gearsystem" },
  { system: "game-gear", value: "gamegear/gearsystem", label: "Gearsystem" },
  { system: "game-gear", value: "gamegear/emulicious", label: "Emulicious" },
  { system: "game-gear", value: "gamegear/genesis-plus-gx", label: "Genesis Plus GX" },
  { system: "game-gear", value: "gamegear/retroarch-gearsystem", label: "RetroArch Gearsystem" },
  { system: "pc-engine", value: "pc-engine/mister", label: "MiSTer PC Engine" },
  { system: "pc-engine", value: "pc-engine/mednafen", label: "Mednafen" },
  { system: "pc-engine", value: "pc-engine/retroarch-beetle-pce", label: "RetroArch Beetle PCE" },
  { system: "pc-engine", value: "pc-engine/mesen2", label: "Mesen 2" },
  { system: "atari-lynx", value: "atari-lynx/handy", label: "Handy" },
  { system: "atari-lynx", value: "atari-lynx/mednafen", label: "Mednafen" },
  { system: "atari-lynx", value: "atari-lynx/retroarch-handy", label: "RetroArch Handy" },
  { system: "wonderswan", value: "wonderswan/mednafen", label: "Mednafen" },
  { system: "wonderswan", value: "wonderswan/ares", label: "ares" },
  { system: "wonderswan", value: "wonderswan/retroarch-beetle-wswan", label: "RetroArch Beetle WonderSwan" },
  { system: "sg-1000", value: "sg-1000/emulicious", label: "Emulicious" },
  { system: "sg-1000", value: "sg-1000/gearsystem", label: "Gearsystem" },
  { system: "sg-1000", value: "sg-1000/genesis-plus-gx", label: "Genesis Plus GX" },
  { system: "colecovision", value: "colecovision/blue-msx", label: "blueMSX" },
  { system: "colecovision", value: "colecovision/gearcoleco", label: "Gearcoleco" },
  { system: "colecovision", value: "colecovision/mame", label: "MAME" },
  { system: "atari-jaguar", value: "atari-jaguar/bigpemu", label: "BigPEmu" },
  { system: "atari-jaguar", value: "atari-jaguar/virtual-jaguar", label: "Virtual Jaguar" },
  { system: "atari-jaguar", value: "atari-jaguar/retroarch-virtual-jaguar", label: "RetroArch Virtual Jaguar" },
  { system: "3do", value: "3do/opera", label: "Opera" },
  { system: "3do", value: "3do/phoenix", label: "Phoenix" },
  { system: "3do", value: "3do/4do", label: "4DO" },
  { system: "dreamcast", value: "dreamcast/flycast", label: "Flycast" },
  { system: "dreamcast", value: "dreamcast/redream", label: "Redream" },
  { system: "dreamcast", value: "dreamcast/mister", label: "MiSTer Dreamcast" },
  { system: "dreamcast", value: "dreamcast/retroarch-flycast", label: "RetroArch Flycast" },
  { system: "saturn", value: "saturn/mister", label: "MiSTer Saturn" },
  { system: "saturn", value: "saturn/mednafen", label: "Mednafen" },
  { system: "saturn", value: "saturn/yabasanshiro", label: "YabaSanshiro" },
  { system: "psx", value: "psx/mister", label: "MiSTer PSX" },
  { system: "psx", value: "psx/retroarch", label: "RetroArch PSX" },
  { system: "ps2", value: "ps2/pcsx2", label: "PCSX2" }
];

export function runtimeProfileOptionsForSystem(systemSlug: string): Array<{ value: string; label: string }> {
  const slug = systemSlug.trim();
  if (!slug) {
    return RUNTIME_PROFILE_OPTIONS.map(({ value, label }) => ({ value, label }));
  }
  return RUNTIME_PROFILE_OPTIONS.filter((option) => option.system === slug).map(({ value, label }) => ({ value, label }));
}
