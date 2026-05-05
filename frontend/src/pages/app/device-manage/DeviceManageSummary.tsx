import type { Device, DeviceConfigSource } from "../../../services/types";

export function DeviceManageSummary({
  device,
  sources,
  syncAll,
  allowedSystems
}: {
  device: Device;
  sources: DeviceConfigSource[];
  syncAll: boolean;
  allowedSystems: string[];
}): JSX.Element {
  const effectiveSystems = device.effectivePolicy?.allowedSystemSlugs ?? (syncAll ? device.reportedSystemSlugs : allowedSystems);
  const blockedCount = device.effectivePolicy?.blocked?.length ?? 0;
  const serviceState = device.service?.freshness || device.service?.status || (device.lastSeenAt ? "seen" : "offline");
  const savePathCount = device.sensors?.savePathCount ?? sources.reduce((count, source) => count + (source.savePaths?.length ?? (source.savePath ? 1 : 0)), 0);
  return (
    <div className="device-manage-summary" aria-label="Device policy summary">
      <div>
        <span>Service</span>
        <strong>{serviceState}</strong>
        <small>{device.service?.lastError || device.service?.lastEvent || "No recent error"}</small>
      </div>
      <div>
        <span>Policy</span>
        <strong>{syncAll ? "Auto" : "Manual"}</strong>
        <small>{effectiveSystems?.length ? `${effectiveSystems.length} consoles allowed` : "No consoles allowed yet"}</small>
      </div>
      <div>
        <span>Sources</span>
        <strong>{sources.length}</strong>
        <small>{savePathCount} save folders reported</small>
      </div>
      <div className={blockedCount > 0 ? "device-manage-summary__warn" : ""}>
        <span>Guard</span>
        <strong>{blockedCount}</strong>
        <small>{blockedCount > 0 ? "blocked unsafe routes" : "no blocked routes"}</small>
      </div>
    </div>
  );
}
