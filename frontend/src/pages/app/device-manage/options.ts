import type { DeviceConfigSource, SaveSystem } from "../../../services/types";

export type SystemGroup = {
  manufacturer: string;
  systems: SaveSystem[];
};

export type DeviceManageCommand = "sync" | "scan" | "deep_scan" | "config_changed";

export const SOURCE_KIND_OPTIONS = [
  { value: "custom", label: "Custom" },
  { value: "retroarch", label: "RetroArch" },
  { value: "mister-fpga", label: "MiSTer FPGA" },
  { value: "steamdeck", label: "Steam Deck" },
  { value: "windows", label: "Windows" },
  { value: "openemu", label: "OpenEmu" },
  { value: "analogue-pocket", label: "Analogue Pocket" }
];

const PROFILE_OPTIONS = [
  { value: "retroarch", label: "RetroArch" },
  { value: "mister", label: "MiSTer" },
  { value: "snes9x", label: "Snes9x" },
  { value: "bsnes", label: "bsnes" },
  { value: "mesen2", label: "Mesen 2" },
  { value: "fceux", label: "FCEUX" },
  { value: "nestopia-ue", label: "Nestopia UE" },
  { value: "mgba", label: "mGBA" },
  { value: "vba-m", label: "VBA-M" },
  { value: "project64", label: "Project64" },
  { value: "mupen-family", label: "Mupen/RMG" },
  { value: "everdrive", label: "EverDrive" },
  { value: "genesis-plus-gx", label: "Genesis Plus GX" },
  { value: "picodrive", label: "PicoDrive" },
  { value: "flycast", label: "Flycast" },
  { value: "redream", label: "Redream" },
  { value: "generic", label: "Generic" }
];

const SYSTEM_PROFILE_OPTIONS: Record<string, Array<{ value: string; label: string }>> = {
  snes: [
    { value: "snes9x", label: "Snes9x" },
    { value: "bsnes", label: "bsnes" },
    { value: "retroarch-snes9x", label: "RetroArch Snes9x" },
    { value: "mesen2", label: "Mesen 2" },
    { value: "higan", label: "higan" }
  ],
  nes: [
    { value: "mesen2", label: "Mesen 2" },
    { value: "fceux", label: "FCEUX" },
    { value: "nestopia-ue", label: "Nestopia UE" },
    { value: "punes", label: "puNES" },
    { value: "retroarch-fceumm", label: "RetroArch FCEUmm" }
  ],
  gba: [
    { value: "mgba", label: "mGBA" },
    { value: "vba-m", label: "VBA-M" },
    { value: "nocashgba", label: "No$GBA" },
    { value: "skyemu", label: "SkyEmu" }
  ],
  n64: [
    { value: "mister", label: "MiSTer" },
    { value: "retroarch", label: "RetroArch" },
    { value: "project64", label: "Project64" },
    { value: "mupen-family", label: "Mupen/RMG" },
    { value: "everdrive", label: "EverDrive" }
  ],
  genesis: [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "picodrive", label: "PicoDrive" },
    { value: "blastem", label: "BlastEm" }
  ],
  "sega-cd": [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "picodrive", label: "PicoDrive" },
    { value: "retroarch-genesis-plus-gx", label: "RetroArch Genesis Plus GX" }
  ],
  "sega-32x": [
    { value: "picodrive", label: "PicoDrive" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "retroarch-picodrive", label: "RetroArch PicoDrive" }
  ],
  "master-system": [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "gearsystem", label: "Gearsystem" },
    { value: "emulicious", label: "Emulicious" },
    { value: "meka", label: "MEKA" }
  ],
  "game-gear": [
    { value: "gearsystem", label: "Gearsystem" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "emulicious", label: "Emulicious" }
  ],
  "pc-engine": [
    { value: "mister", label: "MiSTer" },
    { value: "mednafen", label: "Mednafen" },
    { value: "retroarch-beetle-pce", label: "RetroArch Beetle PCE" },
    { value: "mesen2", label: "Mesen 2" }
  ],
  "atari-lynx": [
    { value: "handy", label: "Handy" },
    { value: "mednafen", label: "Mednafen" },
    { value: "retroarch-handy", label: "RetroArch Handy" }
  ],
  wonderswan: [
    { value: "mednafen", label: "Mednafen" },
    { value: "ares", label: "ares" },
    { value: "retroarch-beetle-wswan", label: "RetroArch Beetle WonderSwan" }
  ],
  "sg-1000": [
    { value: "emulicious", label: "Emulicious" },
    { value: "gearsystem", label: "Gearsystem" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" }
  ],
  colecovision: [
    { value: "blue-msx", label: "blueMSX" },
    { value: "gearcoleco", label: "Gearcoleco" },
    { value: "mame", label: "MAME" }
  ],
  "atari-jaguar": [
    { value: "bigpemu", label: "BigPEmu" },
    { value: "virtual-jaguar", label: "Virtual Jaguar" },
    { value: "retroarch-virtual-jaguar", label: "RetroArch Virtual Jaguar" }
  ],
  "3do": [
    { value: "opera", label: "Opera" },
    { value: "phoenix", label: "Phoenix" },
    { value: "4do", label: "4DO" }
  ],
  dreamcast: [
    { value: "flycast", label: "Flycast" },
    { value: "redream", label: "Redream" },
    { value: "mister", label: "MiSTer" },
    { value: "retroarch-flycast", label: "RetroArch Flycast" }
  ],
  saturn: [
    { value: "mister", label: "MiSTer" },
    { value: "mednafen", label: "Mednafen" },
    { value: "yabasanshiro", label: "Yaba Sanshiro" },
    { value: "yabause", label: "Yabause" }
  ],
  psx: [
    { value: "mister", label: "MiSTer" },
    { value: "retroarch", label: "RetroArch" }
  ],
  ps2: [{ value: "pcsx2", label: "PCSX2" }]
};

export function cloneSource(source: DeviceConfigSource): DeviceConfigSource {
  return {
    ...source,
    savePaths: source.savePaths ? [...source.savePaths] : undefined,
    romPaths: source.romPaths ? [...source.romPaths] : undefined,
    systems: source.systems ? [...source.systems] : undefined,
    unsupportedSystemSlugs: source.unsupportedSystemSlugs ? [...source.unsupportedSystemSlugs] : undefined
  };
}

export function uniqueSourceId(baseId: string, existingIds: Set<string>): string {
  let id = baseId;
  let suffix = 2;
  while (existingIds.has(id)) {
    id = `${baseId}-${suffix}`;
    suffix += 1;
  }
  return id;
}

export function profileLabel(profile: string): string {
  return PROFILE_OPTIONS.find((option) => option.value === profile)?.label
    ?? Object.values(SYSTEM_PROFILE_OPTIONS).flat().find((option) => option.value === profile)?.label
    ?? profile;
}

export function formatSourcePaths(paths: string[]): string {
  if (paths.length === 0) {
    return "no save path";
  }
  if (paths.length === 1) {
    return paths[0];
  }
  return `${paths[0]} + ${paths.length - 1} more`;
}

export function profileOptionsForSystem(systemSlug: string): Array<{ value: string; label: string }> {
  const scoped = SYSTEM_PROFILE_OPTIONS[systemSlug.trim()];
  if (scoped && scoped.length > 0) {
    return scoped;
  }
  return PROFILE_OPTIONS;
}

export function recommendedKindForProfile(profile: string): string {
  if (profile === "mister" || profile === "everdrive") {
    return "mister-fpga";
  }
  if (profile.startsWith("retroarch")) {
    return "retroarch";
  }
  return "custom";
}

export function commandLabel(command: DeviceManageCommand): string {
  switch (command) {
    case "sync":
      return "Sync";
    case "scan":
      return "Scan";
    case "deep_scan":
      return "Deep scan";
    case "config_changed":
      return "Reload config";
    default:
      return command;
  }
}
