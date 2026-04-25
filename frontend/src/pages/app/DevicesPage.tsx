import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { commandDevice, deleteDevice, listDevices } from "../../services/retrosaveApi";
import type { Device, DevicePolicyBlock } from "../../services/types";
import { formatDate } from "../../utils/format";

export function DevicesPage(): JSX.Element {
  const loader = useCallback(() => listDevices(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [commandKey, setCommandKey] = useState<string | null>(null);
  const [commandMessage, setCommandMessage] = useState<string | null>(null);
  const [commandError, setCommandError] = useState<string | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);

  async function onDelete(id: number): Promise<void> {
    const confirmed = window.confirm("Delete this device?");
    if (!confirmed) {
      return;
    }
    await deleteDevice(id);
    if (selectedDevice?.id === id) {
      setSelectedDevice(null);
    }
    reload();
  }

  async function onCommand(device: Device, command: "sync" | "scan" | "deep_scan"): Promise<void> {
    const key = `${device.id}:${command}`;
    setCommandKey(key);
    setCommandMessage(null);
    setCommandError(null);
    try {
      await commandDevice(device.id, command, "devices_page");
      setCommandMessage(`${commandLabel(command)} sent to ${device.displayName}`);
    } catch (err: unknown) {
      setCommandError(err instanceof Error ? err.message : "Command failed");
    } finally {
      setCommandKey(null);
    }
  }

  const devices = data ?? [];

  return (
    <SectionCard
      title="Devices"
      subtitle="A compact helper overview. Open details for full service, config, and folder information."
    >
      {loading ? <LoadingState label="Loading devices..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {commandError ? <ErrorState message={commandError} /> : null}
      {commandMessage ? <p className="success-state">{commandMessage}</p> : null}
      {data ? (
        <>
          <DeviceFleetSummary devices={devices} />
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
                  <button className="btn btn-ghost" type="button" onClick={() => setSelectedDevice(device)}>
                    Details
                  </button>
                  <Link className="btn btn-ghost" to={`/app/devices/${device.id}/manage`}>
                    Manage
                  </Link>
                </div>
              </article>
            ))}
          </div>
        </>
      ) : null}

      {selectedDevice ? (
        <DeviceDetailsModal
          device={selectedDevice}
          commandKey={commandKey}
          onClose={() => setSelectedDevice(null)}
          onCommand={onCommand}
          onDelete={onDelete}
        />
      ) : null}
    </SectionCard>
  );
}

function DeviceFleetSummary({ devices }: { devices: Device[] }): JSX.Element {
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

function DeviceDetailsModal({
  device,
  commandKey,
  onClose,
  onCommand,
  onDelete
}: {
  device: Device;
  commandKey: string | null;
  onClose: () => void;
  onCommand: (device: Device, command: "sync" | "scan" | "deep_scan") => Promise<void>;
  onDelete: (id: number) => Promise<void>;
}): JSX.Element {
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
            <button className="btn btn-ghost btn-danger" type="button" onClick={() => void onDelete(device.id)}>
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
                  { label: "Status", value: formatServiceStatus(device) },
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
                  { label: "Backend URL", value: formatBackendURL(device) },
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

function DeviceServiceBadge({ device }: { device: Device }): JSX.Element {
  const freshness = device.service?.freshness ?? (device.lastSeenAt ? "seen" : "offline");
  return <span className={`device-status-badge device-status-badge--${freshness}`}>{freshness}</span>;
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

function DeviceChipList({ items, emptyLabel }: { items?: string[]; emptyLabel: string }): JSX.Element {
  if (!items || items.length === 0) {
    return <span>{emptyLabel}</span>;
  }
  return (
    <div className="device-chip-list">
      {items.map((item) => (
        <span key={item} className="device-chip">
          {item}
        </span>
      ))}
    </div>
  );
}

function DevicePathList({ items, emptyLabel }: { items?: string[]; emptyLabel: string }): JSX.Element {
  if (!items || items.length === 0) {
    return <span>{emptyLabel}</span>;
  }
  return (
    <ul className="device-path-list">
      {items.map((item) => (
        <li key={item}>
          <code>{item}</code>
        </li>
      ))}
    </ul>
  );
}

function DeviceBlockedList({ items }: { items?: DevicePolicyBlock[] }): JSX.Element {
  if (!items || items.length === 0) {
    return <span>No backend blocks</span>;
  }
  return (
    <div className="device-chip-list">
      {items.map((item) => (
        <span key={`${item.system}-${item.sourceId ?? "device"}`} className="device-chip device-chip--blocked" title={item.reason}>
          {item.system}
        </span>
      ))}
    </div>
  );
}

function DeviceConfigSourceList({ device }: { device: Device }): JSX.Element | null {
  const sources = device.configSources ?? [];
  if (sources.length === 0) {
    return null;
  }
  return (
    <section className="device-config-sources">
      <div className="device-config-sources__header">
        <h3>Helper config sources</h3>
        <span>{device.configReportedAt ? `Reported ${formatDeviceTimestamp(device.configReportedAt)}` : "Reported by helper"}</span>
      </div>
      {device.configRevision ? <p className="device-config-sources__revision">Revision: {device.configRevision}</p> : null}
      <div className="device-config-sources__list">
        {sources.map((source) => (
          <article key={source.id} className="device-config-source">
            <strong>{source.label || source.id}</strong>
            <p>
              {[source.kind, source.profile, source.origin].filter(Boolean).join(" / ") || "Unknown source"}
              {source.managed ? " · backend-managed" : " · helper-managed"}
            </p>
            <dl>
              <div>
                <dt>Save paths</dt>
                <dd>
                  <DevicePathList items={source.savePaths ?? (source.savePath ? [source.savePath] : undefined)} emptyLabel="Not reported" />
                </dd>
              </div>
              <div>
                <dt>ROM paths</dt>
                <dd>
                  <DevicePathList items={source.romPaths ?? (source.romPath ? [source.romPath] : undefined)} emptyLabel="Not reported" />
                </dd>
              </div>
              <div>
                <dt>Configured systems</dt>
                <dd>
                  <DeviceChipList items={source.systems} emptyLabel="No supported systems reported" />
                </dd>
              </div>
              <div>
                <dt>Unsupported</dt>
                <dd>
                  <DeviceChipList items={source.unsupportedSystemSlugs} emptyLabel="None" />
                </dd>
              </div>
            </dl>
          </article>
        ))}
      </div>
    </section>
  );
}

function compactSystems(items?: string[]): string[] | undefined {
  if (!items || items.length <= 4) {
    return items;
  }
  return [...items.slice(0, 4), `+${items.length - 4}`];
}

function formatHelperHeadline(device: Device): string {
  const parts = [device.helperName, device.helperVersion, device.deviceType].filter(Boolean);
  if (parts.length === 0) {
    return "Unknown helper";
  }
  return parts.join(" · ");
}

function formatServiceMode(device: Device): string {
  const service = device.service;
  if (!service) {
    return "No daemon heartbeat";
  }
  return [service.mode, service.loop].filter(Boolean).join(" · ") || "Daemon reported";
}

function formatServiceStatus(device: Device): JSX.Element {
  const service = device.service;
  if (!service) {
    return <span>No daemon heartbeat</span>;
  }
  const text = [service.freshness, service.status].filter(Boolean).join(" / ") || "Unknown";
  return <span className={`device-status-text device-status-text--${service.freshness ?? "unknown"}`}>{text}</span>;
}

function formatSeconds(value?: number): string {
  if (!value || value <= 0) {
    return "Default";
  }
  return `${value}s`;
}

function formatLastSyncStats(device: Device): string {
  const stats = device.sensors?.lastSync;
  if (!stats) {
    return "No counters";
  }
  return `${stats.scanned} scanned, ${stats.uploaded} uploaded, ${stats.downloaded} downloaded, ${stats.errors} errors`;
}

function formatLastSyncStatsShort(device: Device): string {
  const stats = device.sensors?.lastSync;
  if (!stats) {
    return formatDeviceTimestamp(device.lastSyncedAt);
  }
  if (stats.errors > 0) {
    return `${stats.errors} errors`;
  }
  return `${stats.uploaded} up / ${stats.downloaded} down`;
}

function commandLabel(command: "sync" | "scan" | "deep_scan"): string {
  switch (command) {
    case "deep_scan":
      return "Deep scan";
    case "scan":
      return "Scan";
    default:
      return "Sync";
  }
}

function formatConsolePolicy(device: Device): string {
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

function formatAppPassword(device: Device): string {
  if (!device.boundAppPasswordName) {
    if (device.configGlobal?.appPasswordConfigured) {
      return "Configured on helper";
    }
    return "Not bound";
  }
  const suffix = device.boundAppPasswordLastFour ? ` (${device.boundAppPasswordLastFour})` : "";
  return `${device.boundAppPasswordName}${suffix}`;
}

function formatBackendURL(device: Device): JSX.Element | string {
  const global = device.configGlobal;
  if (!global) {
    return "Unknown";
  }
  const value = global.baseUrl || [global.url, global.port ? `:${global.port}` : ""].filter(Boolean).join("");
  return value ? <code>{value}</code> : "Unknown";
}

function formatBoolean(value?: boolean): string {
  return value ? "Enabled" : "Disabled";
}

function formatDeviceTimestamp(value?: string): string {
  if (!value) {
    return "Unknown";
  }
  return formatDate(value);
}
