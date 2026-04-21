import { FormEvent, useCallback, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { deleteDevice, listDevices, renameDevice } from "../../services/retrosaveApi";
import { formatDate } from "../../utils/format";

export function DevicesPage(): JSX.Element {
  const loader = useCallback(() => listDevices(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [aliasDrafts, setAliasDrafts] = useState<Record<number, string>>({});

  async function onRename(event: FormEvent, id: number): Promise<void> {
    event.preventDefault();
    const alias = (aliasDrafts[id] ?? "").trim();
    if (!alias) {
      return;
    }
    await renameDevice(id, alias);
    reload();
  }

  async function onDelete(id: number): Promise<void> {
    await deleteDevice(id);
    reload();
  }

  return (
    <SectionCard title="Devices" subtitle="Beheer trusted apparaten die syncen met de service.">
      {loading ? <LoadingState label="Devices laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <ul className="plain-list">
          {data.map((device) => (
            <li key={device.id} className="list-row">
              <div>
                <strong>{device.displayName}</strong>
                <p>{device.deviceType}</p>
                <small>Last synced: {formatDate(device.lastSyncedAt)}</small>
              </div>
              <form className="inline-actions" onSubmit={(event) => void onRename(event, device.id)}>
                <input
                  value={aliasDrafts[device.id] ?? ""}
                  onChange={(event) =>
                    setAliasDrafts((prev) => ({
                      ...prev,
                      [device.id]: event.target.value
                    }))
                  }
                  placeholder="Alias"
                />
                <button className="btn btn-ghost" type="submit">
                  Rename
                </button>
                <button className="btn btn-ghost" type="button" onClick={() => void onDelete(device.id)}>
                  Delete
                </button>
              </form>
            </li>
          ))}
        </ul>
      ) : null}
    </SectionCard>
  );
}
