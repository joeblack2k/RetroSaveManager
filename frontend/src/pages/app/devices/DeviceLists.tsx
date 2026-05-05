import type { Device, DevicePolicyBlock } from "../../../services/types";
import { formatDeviceTimestamp } from "./helpers";

export function DeviceChipList({ items, emptyLabel }: { items?: string[]; emptyLabel: string }): JSX.Element {
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

export function DevicePathList({ items, emptyLabel }: { items?: string[]; emptyLabel: string }): JSX.Element {
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

export function DeviceBlockedList({ items }: { items?: DevicePolicyBlock[] }): JSX.Element {
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

export function DeviceConfigSourceList({ device }: { device: Device }): JSX.Element | null {
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
