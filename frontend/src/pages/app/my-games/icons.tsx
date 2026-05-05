import { ChevronDown, ChevronRight, Download, Folder, Gamepad2, HardDrive, Info, Sparkles, Trash2 } from "lucide-react";

export function FolderIcon(): JSX.Element {
  return <Folder className="treegrid-folder-icon" aria-hidden="true" />;
}

export function ChevronIcon({ expanded }: { expanded: boolean }): JSX.Element {
  const Icon = expanded ? ChevronDown : ChevronRight;
  return <Icon className="treegrid-chevron-icon" aria-hidden="true" />;
}

export function SystemGlyph({ systemSlug, fallbackLabel }: { systemSlug: string; fallbackLabel: string }): JSX.Element {
  switch (systemSlug) {
    case "psx":
    case "ps2":
    case "psp":
    case "psvita":
    case "n64":
    case "nds":
    case "nes":
    case "snes":
    case "gameboy":
    case "gba":
    case "wii":
    case "atari-lynx":
    case "wonderswan":
      return <Gamepad2 className="treegrid-system-glyph" aria-hidden="true" />;
    case "dreamcast":
    case "game-gear":
    case "genesis":
    case "master-system":
    case "pc-engine":
    case "sega-32x":
    case "sega-cd":
    case "sg-1000":
      return <HardDrive className="treegrid-system-glyph" aria-hidden="true" />;
    default:
      return <span className="treegrid-platform-badge__label">{fallbackLabel}</span>;
  }
}

export function CheatIcon(): JSX.Element {
  return <Sparkles className="treegrid-inline-icon" aria-hidden="true" />;
}

export function DetailsIcon(): JSX.Element {
  return <Info className="treegrid-inline-icon" aria-hidden="true" />;
}

export function DownloadIcon(): JSX.Element {
  return <Download className="treegrid-inline-icon" aria-hidden="true" />;
}

export function DeleteIcon(): JSX.Element {
  return <Trash2 className="treegrid-inline-icon" aria-hidden="true" />;
}
