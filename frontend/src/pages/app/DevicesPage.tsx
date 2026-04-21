import { useCallback } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { deleteDevice, listDevices } from "../../services/retrosaveApi";
import { formatDate } from "../../utils/format";

export function DevicesPage(): JSX.Element {
  const loader = useCallback(() => listDevices(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);

  async function onDelete(id: number): Promise<void> {
    const confirmed = window.confirm("Device verwijderen?");
    if (!confirmed) {
      return;
    }
    await deleteDevice(id);
    reload();
  }

  return (
    <SectionCard title="Devices" subtitle="Beheer gekoppelde helper-devices en sync policies per console.">
      {loading ? <LoadingState label="Devices laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <ul className="plain-list">
          {data.map((device) => (
            <li key={device.id} className="list-row">
              <div>
                <strong>{device.displayName}</strong>
                <p>
                  {device.deviceType} · {device.fingerprint}
                </p>
                <p>
                  App password:{" "}
                  {device.boundAppPasswordName
                    ? `${device.boundAppPasswordName} (${device.boundAppPasswordLastFour ?? "----"})`
                    : "none"}
                </p>
                <p>
                  Console sync: {device.syncAll ? "All" : (device.allowedSystemSlugs ?? []).join(", ") || "None"}
                </p>
                <small>Last synced: {formatDate(device.lastSyncedAt)}</small>
              </div>
              <div className="inline-actions">
                <Link className="btn btn-ghost" to={`/app/devices/${device.id}/manage`}>
                  Manage
                </Link>
                <button className="btn btn-ghost" type="button" onClick={() => void onDelete(device.id)}>
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
    </SectionCard>
  );
}
