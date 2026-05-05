import type { Device } from "../../../services/types";

export function DeviceFleetSummary({ devices }: { devices: Device[] }): JSX.Element {
  const online = devices.filter((device) => device.service?.freshness === "online").length;
  const degraded = devices.filter((device) => device.service?.freshness === "degraded").length;
  const staleOrOffline = devices.filter((device) => {
    const freshness = device.service?.freshness;
    return freshness === "stale" || freshness === "offline";
  }).length;
  const folders = devices.reduce((total, device) => total + (device.sensors?.savePathCount ?? device.syncPaths?.length ?? 0), 0);

  return (
    <div className="device-fleet-summary" aria-label="Device fleet summary">
      <div>
        <span>Helpers</span>
        <strong>{devices.length}</strong>
      </div>
      <div>
        <span>Online</span>
        <strong>{online}</strong>
      </div>
      <div>
        <span>Degraded</span>
        <strong>{degraded}</strong>
      </div>
      <div>
        <span>Stale/offline</span>
        <strong>{staleOrOffline}</strong>
      </div>
      <div>
        <span>Save folders</span>
        <strong>{folders}</strong>
      </div>
    </div>
  );
}
