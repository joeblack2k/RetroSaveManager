import type { Device } from "../../../services/types";
import { formatDate } from "../../../utils/format";
import type { DeviceCommand } from "./types";

export function compactSystems(items?: string[]): string[] | undefined {
  if (!items || items.length <= 4) {
    return items;
  }
  return [...items.slice(0, 4), `+${items.length - 4}`];
}

export function formatHelperHeadline(device: Device): string {
  const parts = [device.helperName, device.helperVersion, device.deviceType].filter(Boolean);
  if (parts.length === 0) {
    return "Unknown helper";
  }
  return parts.join(" · ");
}

export function formatServiceMode(device: Device): string {
  const service = device.service;
  if (!service) {
    return "No daemon heartbeat";
  }
  return [service.mode, service.loop].filter(Boolean).join(" · ") || "Daemon reported";
}

export function formatSeconds(value?: number): string {
  if (!value || value <= 0) {
    return "Default";
  }
  return `${value}s`;
}

export function formatLastSyncStats(device: Device): string {
  const stats = device.sensors?.lastSync;
  if (!stats) {
    return "No counters";
  }
  return `${stats.scanned} scanned, ${stats.uploaded} uploaded, ${stats.downloaded} downloaded, ${stats.errors} errors`;
}

export function formatLastSyncStatsShort(device: Device): string {
  const stats = device.sensors?.lastSync;
  if (!stats) {
    return formatDeviceTimestamp(device.lastSyncedAt);
  }
  if (stats.errors > 0) {
    return `${stats.errors} errors`;
  }
  return `${stats.uploaded} up / ${stats.downloaded} down`;
}

export function commandLabel(command: DeviceCommand): string {
  switch (command) {
    case "deep_scan":
      return "Deep scan";
    case "scan":
      return "Scan";
    default:
      return "Sync";
  }
}

export function formatConsolePolicy(device: Device): string {
  if (device.syncAll) {
    if (device.effectivePolicy?.mode === "source-scoped-all") {
      return "All systems allowed by helper config";
    }
    return "All supported systems";
  }
  if (!device.allowedSystemSlugs || device.allowedSystemSlugs.length === 0) {
    return "No systems allowed";
  }
  return device.allowedSystemSlugs.join(", ");
}

export function formatAppPassword(device: Device): string {
  if (!device.boundAppPasswordName) {
    if (device.configGlobal?.appPasswordConfigured) {
      return "Configured on helper";
    }
    return "Not bound";
  }
  const suffix = device.boundAppPasswordLastFour ? ` (${device.boundAppPasswordLastFour})` : "";
  return `${device.boundAppPasswordName}${suffix}`;
}

export function backendURLValue(device: Device): string {
  const global = device.configGlobal;
  if (!global) {
    return "";
  }
  return global.baseUrl || [global.url, global.port ? `:${global.port}` : ""].filter(Boolean).join("");
}

export function formatBoolean(value?: boolean): string {
  return value ? "Enabled" : "Disabled";
}

export function formatDeviceTimestamp(value?: string): string {
  if (!value) {
    return "Unknown";
  }
  return formatDate(value);
}
