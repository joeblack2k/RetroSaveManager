import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getDevice, listSaveSystems, updateDevice } from "../../services/retrosaveApi";
import type { SaveSystem } from "../../services/types";

type SystemGroup = {
  manufacturer: string;
  systems: SaveSystem[];
};

export function DeviceManagePage(): JSX.Element {
  const params = useParams<{ deviceId: string }>();
  const deviceId = Number.parseInt(params.deviceId ?? "0", 10);

  const loader = useCallback(async () => {
    if (!Number.isFinite(deviceId) || deviceId <= 0) {
      throw new Error("Ongeldig device id");
    }
    const [device, systems] = await Promise.all([getDevice(deviceId), listSaveSystems()]);
    return { device, systems };
  }, [deviceId]);

  const { loading, error, data, reload } = useAsyncData(loader, [deviceId]);

  const [alias, setAlias] = useState("");
  const [syncAll, setSyncAll] = useState(true);
  const [allowedSystems, setAllowedSystems] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!data) {
      return;
    }
    setAlias(data.device.alias ?? "");
    setSyncAll(data.device.syncAll);
    setAllowedSystems((data.device.allowedSystemSlugs ?? []).slice().sort());
  }, [data]);

  const groups = useMemo<SystemGroup[]>(() => {
    const grouped = new Map<string, SaveSystem[]>();
    for (const item of data?.systems ?? []) {
      const slug = item.slug?.trim();
      if (!slug) {
        continue;
      }
      const manufacturer = (item.manufacturer || "Other").trim() || "Other";
      const current = grouped.get(manufacturer) ?? [];
      current.push(item);
      grouped.set(manufacturer, current);
    }

    const out: SystemGroup[] = [];
    for (const [manufacturer, systems] of grouped.entries()) {
      systems.sort((a, b) => a.name.localeCompare(b.name));
      out.push({ manufacturer, systems });
    }
    out.sort((a, b) => {
      if (a.manufacturer === "Other") {
        return 1;
      }
      if (b.manufacturer === "Other") {
        return -1;
      }
      return a.manufacturer.localeCompare(b.manufacturer);
    });
    return out;
  }, [data]);

  function toggleAllowedSystem(slug: string): void {
    setAllowedSystems((previous) => {
      if (previous.includes(slug)) {
        return previous.filter((value) => value !== slug);
      }
      return [...previous, slug].sort();
    });
  }

  async function onSave(): Promise<void> {
    if (!Number.isFinite(deviceId) || deviceId <= 0) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    setSaveMessage(null);
    try {
      await updateDevice(deviceId, {
        alias,
        syncAll,
        allowedSystemSlugs: syncAll ? [] : allowedSystems
      });
      setSaveMessage("Device settings opgeslagen");
      reload();
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : "Opslaan mislukt");
    } finally {
      setSaving(false);
    }
  }

  return (
    <SectionCard title="Manage Device" subtitle="Hernoem het device en beheer welke consoles mogen syncen.">
      {loading ? <LoadingState label="Device laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {saveError ? <ErrorState message={saveError} /> : null}
      {saveMessage ? <p className="success-state">{saveMessage}</p> : null}

      {data ? (
        <div className="stack">
          <p>
            <strong>Device:</strong> {data.device.displayName} ({data.device.deviceType} · {data.device.fingerprint})
          </p>

          <label className="field">
            <span>Alias</span>
            <input
              value={alias}
              onChange={(event) => setAlias(event.target.value)}
              placeholder="Bijv. SteamDeck"
            />
          </label>

          <label className="sync-toggle-row">
            <input type="checkbox" checked={syncAll} onChange={(event) => setSyncAll(event.target.checked)} />
            <span>Console sync: All</span>
          </label>

          {!syncAll ? (
            <div className="stack compact">
              {groups.map((group) => (
                <details key={group.manufacturer} className="device-group" open>
                  <summary>
                    <strong>{group.manufacturer}</strong>
                    <span>{group.systems.length} consoles</span>
                  </summary>
                  <div className="device-group__list">
                    {group.systems.map((system) => {
                      const slug = system.slug ?? "";
                      return (
                        <label key={slug} className="sync-option-row">
                          <input
                            type="checkbox"
                            checked={allowedSystems.includes(slug)}
                            onChange={() => toggleAllowedSystem(slug)}
                          />
                          <span>{system.name}</span>
                        </label>
                      );
                    })}
                  </div>
                </details>
              ))}
            </div>
          ) : null}

          <div className="inline-actions">
            <button className="btn btn-primary" type="button" onClick={() => void onSave()} disabled={saving}>
              {saving ? "Saving..." : "Save"}
            </button>
            <Link className="btn btn-ghost" to="/app/devices">
              Back
            </Link>
          </div>
        </div>
      ) : null}
    </SectionCard>
  );
}
