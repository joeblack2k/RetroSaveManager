import { Link } from "react-router-dom";
import type { Device } from "../../../services/types";
import { compactSystems, formatDeviceTimestamp, formatHelperHeadline, formatLastSyncStatsShort } from "./helpers";
import type { DeviceCommandHandler } from "./types";
import { DeviceChipList } from "./DeviceLists";
import { DeviceServiceBadge } from "./DeviceServiceBadge";

type DeviceCompactListProps = {
  devices: Device[];
  commandKey: string | null;
  onCommand: DeviceCommandHandler;
  onSelectDevice: (device: Device) => void;
};

export function DeviceCompactList({ devices, commandKey, onCommand, onSelectDevice }: DeviceCompactListProps): JSX.Element {
  return (
    <div className="device-compact-list" role="list">
      {devices.map((device) => (
        <article key={device.id} className="device-compact-row" role="listitem">
          <div className="device-compact-row__identity">
            <div className="device-title-row">
              <strong>{device.displayName}</strong>
              <DeviceServiceBadge device={device} />
            </div>
            <p>{formatHelperHeadline(device)}</p>
            <small>
              {device.hostname || "Unknown host"} · {device.lastSeenIp || "No IP"}
            </small>
          </div>

          <div className="device-compact-row__metric">
            <span>Heartbeat</span>
            <strong>{formatDeviceTimestamp(device.service?.lastHeartbeatAt ?? device.lastSeenAt)}</strong>
          </div>

          <div className="device-compact-row__metric">
            <span>Sync</span>
            <strong>{formatLastSyncStatsShort(device)}</strong>
          </div>

          <div className="device-compact-row__systems">
            <span>Systems</span>
            <DeviceChipList
              items={compactSystems(device.effectivePolicy?.allowedSystemSlugs ?? (device.syncAll ? device.reportedSystemSlugs : device.allowedSystemSlugs))}
              emptyLabel={device.syncAll ? "All reported" : "None"}
            />
          </div>

          <div className="device-compact-row__actions">
            <button
              className="btn btn-ghost"
              type="button"
              disabled={commandKey === `${device.id}:sync`}
              onClick={() => void onCommand(device, "sync")}
            >
              Sync
            </button>
            <button className="btn btn-ghost" type="button" onClick={() => onSelectDevice(device)}>
              Details
            </button>
            <Link className="btn btn-ghost" to={`/app/devices/${device.id}/manage`}>
              Manage
            </Link>
          </div>
        </article>
      ))}
    </div>
  );
}
