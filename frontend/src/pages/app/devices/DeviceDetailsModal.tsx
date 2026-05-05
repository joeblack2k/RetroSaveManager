import { Link } from "react-router-dom";
import type { Device } from "../../../services/types";
import {
  backendURLValue,
  formatAppPassword,
  formatBoolean,
  formatConsolePolicy,
  formatDeviceTimestamp,
  formatHelperHeadline,
  formatLastSyncStats,
  formatSeconds,
  formatServiceMode
} from "./helpers";
import type { DeviceCommandHandler } from "./types";
import { DeviceBlockedList, DeviceChipList, DeviceConfigSourceList, DevicePathList } from "./DeviceLists";

type DeviceDetailsModalProps = {
  device: Device;
  commandKey: string | null;
  onClose: () => void;
  onCommand: DeviceCommandHandler;
  onRequestDelete: (device: Device) => void;
};

export function DeviceDetailsModal({
  device,
  commandKey,
  onClose,
  onCommand,
  onRequestDelete
}: DeviceDetailsModalProps): JSX.Element {
  return (
    <div className="treegrid-modal-backdrop" role="presentation" onClick={onClose}>
      <div
        className="treegrid-modal treegrid-modal--wide device-detail-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="device-detail-title"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="treegrid-modal__header">
          <div>
            <h2 id="device-detail-title">{device.displayName}</h2>
            <p>
              {formatHelperHeadline(device)} · {device.hostname || "Unknown host"}
            </p>
          </div>
          <button className="treegrid-modal__close" type="button" onClick={onClose} aria-label="Close device details">
            Close
          </button>
        </header>

        <div className="treegrid-modal__body device-detail-body">
          <div className="device-detail-actions">
            <button
              className="btn btn-ghost"
              type="button"
              disabled={commandKey === `${device.id}:sync`}
              onClick={() => void onCommand(device, "sync")}
            >
              Sync now
            </button>
            <button
              className="btn btn-ghost"
              type="button"
              disabled={commandKey === `${device.id}:scan`}
              onClick={() => void onCommand(device, "scan")}
            >
              Scan
            </button>
            <button
              className="btn btn-ghost"
              type="button"
              disabled={commandKey === `${device.id}:deep_scan`}
              onClick={() => void onCommand(device, "deep_scan")}
            >
              Deep scan
            </button>
            <Link className="btn btn-ghost" to={`/app/devices/${device.id}/manage`}>
              Manage config
            </Link>
            <button className="btn btn-ghost btn-danger" type="button" onClick={() => onRequestDelete(device)}>
              Delete
            </button>
          </div>

          <div className="device-card__grid device-detail-grid">
            <DeviceMetaBlock
              title="Identity"
              rows={[
                { label: "Type", value: device.deviceType },
                { label: "Fingerprint", value: <code>{device.fingerprint}</code> },
                { label: "Hostname", value: device.hostname || "Unknown" },
                { label: "Platform", value: device.platform || "Unknown" }
              ]}
            />
            <DeviceMetaBlock
              title="Helper"
              rows={[
                { label: "Client", value: device.helperName || "Unknown" },
                { label: "Version", value: device.helperVersion || "Unknown" },
                { label: "Service", value: formatServiceMode(device) },
                { label: "Last seen", value: formatDeviceTimestamp(device.lastSeenAt) },
                { label: "Last synced", value: formatDeviceTimestamp(device.lastSyncedAt) }
              ]}
            />
            {device.service ? (
              <DeviceMetaBlock
                title="Service"
                rows={[
                  { label: "Status", value: <DeviceServiceStatus device={device} /> },
                  { label: "Heartbeat", value: formatSeconds(device.service.heartbeatInterval) },
                  { label: "Reconcile", value: formatSeconds(device.service.reconcileInterval) },
                  { label: "Last heartbeat", value: formatDeviceTimestamp(device.service.lastHeartbeatAt) },
                  { label: "Last event", value: device.service.lastEvent || "None" },
                  { label: "Cycles", value: String(device.service.syncCycles ?? 0) },
                  { label: "Last error", value: device.service.lastError || "None", compact: true }
                ]}
              />
            ) : null}
            {device.sensors ? (
              <DeviceMetaBlock
                title="Sensors"
                rows={[
                  { label: "Config hash", value: device.sensors.configHash ? <code>{device.sensors.configHash}</code> : "Unknown", compact: true },
                  { label: "Config readable", value: formatBoolean(device.sensors.configReadable) },
                  { label: "Sources", value: String(device.sensors.sourceCount ?? 0) },
                  { label: "Save paths", value: String(device.sensors.savePathCount ?? 0) },
                  { label: "ROM paths", value: String(device.sensors.romPathCount ?? 0) },
                  { label: "Sync lock", value: device.sensors.syncLockPresent ? "Present" : "Clear" },
                  { label: "Last sync", value: formatLastSyncStats(device) }
                ]}
              />
            ) : null}
            {device.configGlobal ? (
              <DeviceMetaBlock
                title="Config"
                rows={[
                  { label: "Backend URL", value: <BackendURL device={device} /> },
                  { label: "Root", value: device.configGlobal.root ? <code>{device.configGlobal.root}</code> : "Unknown" },
                  { label: "State dir", value: device.configGlobal.stateDir ? <code>{device.configGlobal.stateDir}</code> : "Unknown" },
                  { label: "Watch", value: formatBoolean(device.configGlobal.watch) },
                  { label: "Interval", value: device.configGlobal.watchInterval ? `${device.configGlobal.watchInterval}s` : "Default" },
                  { label: "Force upload", value: formatBoolean(device.configGlobal.forceUpload) },
                  { label: "Dry run", value: formatBoolean(device.configGlobal.dryRun) },
                  { label: "Route prefix", value: device.configGlobal.routePrefix || "None" }
                ]}
              />
            ) : null}
            <DeviceMetaBlock
              title="Network"
              rows={[
                { label: "IP", value: device.lastSeenIp || "Unknown" },
                { label: "User-Agent", value: device.lastSeenUserAgent || "Unknown", compact: true }
              ]}
            />
            <DeviceMetaBlock
              title="Sync"
              rows={[
                { label: "Console policy", value: formatConsolePolicy(device) },
                {
                  label: "Effective systems",
                  value: (
                    <DeviceChipList
                      items={device.effectivePolicy?.allowedSystemSlugs ?? (device.syncAll ? undefined : device.allowedSystemSlugs)}
                      emptyLabel={device.syncAll ? "All supported systems" : "No systems allowed"}
                    />
                  )
                },
                { label: "Blocked systems", value: <DeviceBlockedList items={device.effectivePolicy?.blocked} /> },
                { label: "Reported systems", value: <DeviceChipList items={device.reportedSystemSlugs} emptyLabel="No systems reported" /> },
                { label: "Sync folders", value: <DevicePathList items={device.syncPaths} emptyLabel="No folders reported" /> }
              ]}
            />
            <DeviceMetaBlock
              title="Security"
              rows={[
                { label: "App password", value: formatAppPassword(device) },
                { label: "Created", value: formatDeviceTimestamp(device.createdAt) }
              ]}
            />
          </div>
          <DeviceConfigSourceList device={device} />
        </div>
      </div>
    </div>
  );
}

function DeviceMetaBlock({
  title,
  rows
}: {
  title: string;
  rows: Array<{ label: string; value: JSX.Element | string; compact?: boolean }>;
}): JSX.Element {
  return (
    <section className="device-meta-block">
      <h3>{title}</h3>
      <dl className="device-meta-list">
        {rows.map((row) => (
          <div key={`${title}-${row.label}`} className={row.compact ? "device-meta-row device-meta-row--compact" : "device-meta-row"}>
            <dt>{row.label}</dt>
            <dd>{row.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function DeviceServiceStatus({ device }: { device: Device }): JSX.Element {
  const service = device.service;
  if (!service) {
    return <span>No daemon heartbeat</span>;
  }
  const text = [service.freshness, service.status].filter(Boolean).join(" / ") || "Unknown";
  return <span className={`device-status-text device-status-text--${service.freshness ?? "unknown"}`}>{text}</span>;
}

function BackendURL({ device }: { device: Device }): JSX.Element | string {
  const value = backendURLValue(device);
  return value ? <code>{value}</code> : "Unknown";
}
