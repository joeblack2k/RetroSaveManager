import type { Device } from "../../../services/types";

export function DeviceServiceBadge({ device }: { device: Device }): JSX.Element {
  const freshness = device.service?.freshness ?? (device.lastSeenAt ? "seen" : "offline");
  return <span className={`device-status-badge device-status-badge--${freshness}`}>{freshness}</span>;
}
