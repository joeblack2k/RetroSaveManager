import { useCallback } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { deleteDevice, listDevices } from "../../services/retrosaveApi";
import type { Device } from "../../services/types";
import { formatDate } from "../../utils/format";

export function DevicesPage(): JSX.Element {
  const loader = useCallback(() => listDevices(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);

  async function onDelete(id: number): Promise<void> {
    const confirmed = window.confirm("Delete this device?");
    if (!confirmed) {
      return;
    }
    await deleteDevice(id);
    reload();
  }

  return (
    <SectionCard
      title="Devices"
      subtitle="Helpers report identity, network, and sync scope here so we can see exactly what each client is doing."
    >
      {loading ? <LoadingState label="Loading devices..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <ul className="plain-list device-list">
          {data.map((device) => (
            <li key={device.id} className="device-card">
              <div className="device-card__header">
                <div className="device-card__title-block">
                  <strong>{device.displayName}</strong>
                  <p>
                    {formatHelperHeadline(device)}
                  </p>
                </div>
                <div className="inline-actions">
                  <Link className="btn btn-ghost" to={`/app/devices/${device.id}/manage`}>
                    Manage
                  </Link>
                  <button className="btn btn-ghost btn-danger" type="button" onClick={() => void onDelete(device.id)}>
                    Delete
                  </button>
                </div>
              </div>

              <div className="device-card__grid">
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
                    { label: "Last seen", value: formatDeviceTimestamp(device.lastSeenAt) },
                    { label: "Last synced", value: formatDeviceTimestamp(device.lastSyncedAt) }
                  ]}
                />
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
                      label: "Reported systems",
                      value: <DeviceChipList items={device.reportedSystemSlugs} emptyLabel="No systems reported" />
                    },
                    {
                      label: "Sync folders",
                      value: <DevicePathList items={device.syncPaths} emptyLabel="No folders reported" />
                    }
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
            </li>
          ))}
        </ul>
      ) : null}
    </SectionCard>
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

function formatHelperHeadline(device: Device): string {
  const parts = [device.helperName, device.helperVersion, device.deviceType].filter(Boolean);
  if (parts.length === 0) {
    return "Unknown helper";
  }
  return parts.join(" · ");
}

function formatConsolePolicy(device: Device): string {
  if (device.syncAll) {
    return "All supported systems";
  }
  if (!device.allowedSystemSlugs || device.allowedSystemSlugs.length === 0) {
    return "No systems allowed";
  }
  return device.allowedSystemSlugs.join(", ");
}

function formatAppPassword(device: Device): string {
  if (!device.boundAppPasswordName) {
    return "Not bound";
  }
  const suffix = device.boundAppPasswordLastFour ? ` (${device.boundAppPasswordLastFour})` : "";
  return `${device.boundAppPasswordName}${suffix}`;
}

function formatDeviceTimestamp(value?: string): string {
  if (!value) {
    return "Unknown";
  }
  return formatDate(value);
}
