import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import { commandDevice, getDevice, listSaveSystems, updateDevice } from "../../services/retrosaveApi";
import type { DeviceConfigSource, DevicePolicyBlock, SaveSystem } from "../../services/types";
import { DeviceManageSummary } from "./device-manage/DeviceManageSummary";
import { DeviceSourceEditor } from "./device-manage/DeviceSourceEditor";
import { SystemPolicySelector } from "./device-manage/SystemPolicySelector";
import {
  cloneSource,
  commandLabel,
  profileLabel,
  profileOptionsForSystem,
  recommendedKindForProfile,
  uniqueSourceId,
  type DeviceManageCommand,
  type SystemGroup
} from "./device-manage/options";

export function DeviceManagePage(): JSX.Element {
  const params = useParams<{ deviceId: string }>();
  const deviceId = Number.parseInt(params.deviceId ?? "0", 10);

  const loader = useCallback(async () => {
    if (!Number.isFinite(deviceId) || deviceId <= 0) {
      throw new Error("Invalid device id");
    }
    const [device, systems] = await Promise.all([getDevice(deviceId), listSaveSystems()]);
    return { device, systems };
  }, [deviceId]);

  const { loading, error, data, reload } = useAsyncData(loader, [deviceId]);

  const [alias, setAlias] = useState("");
  const [syncAll, setSyncAll] = useState(true);
  const [allowedSystems, setAllowedSystems] = useState<string[]>([]);
  const [editableSources, setEditableSources] = useState<DeviceConfigSource[]>([]);
  const [sourcesDirty, setSourcesDirty] = useState(false);
  const [draftSystemSlug, setDraftSystemSlug] = useState("");
  const [draftKind, setDraftKind] = useState("custom");
  const [draftProfile, setDraftProfile] = useState("retroarch");
  const [draftSavePath, setDraftSavePath] = useState("");
  const [draftRomPath, setDraftRomPath] = useState("");
  const [draftLabel, setDraftLabel] = useState("");
  const [saving, setSaving] = useState(false);
  const [commandKey, setCommandKey] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!data) {
      return;
    }
    setAlias(data.device.alias ?? "");
    setSyncAll(data.device.syncAll);
    setAllowedSystems((data.device.allowedSystemSlugs ?? []).slice().sort());
    setEditableSources((data.device.configSources ?? []).map(cloneSource));
    setSourcesDirty(false);
    const firstSystem = data.systems.find((system) => system.slug)?.slug ?? "";
    setDraftSystemSlug(firstSystem);
  }, [data]);

  const sourceScopedAllowed = useMemo(() => {
    const allowed = new Set<string>();
    for (const source of data?.device.effectivePolicy?.sources ?? []) {
      for (const slug of source.allowedSystemSlugs ?? []) {
        allowed.add(slug);
      }
    }
    return allowed;
  }, [data]);

  const blockedReasons = useMemo(() => {
    const blocked = new Map<string, DevicePolicyBlock>();
    for (const item of data?.device.effectivePolicy?.blocked ?? []) {
      if (!blocked.has(item.system)) {
        blocked.set(item.system, item);
      }
    }
    return blocked;
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

  const draftProfileOptions = useMemo(() => profileOptionsForSystem(draftSystemSlug), [draftSystemSlug]);

  useEffect(() => {
    if (draftProfileOptions.length === 0) {
      return;
    }
    if (!draftProfileOptions.some((option) => option.value === draftProfile)) {
      const nextProfile = draftProfileOptions[0].value;
      setDraftProfile(nextProfile);
      setDraftKind(recommendedKindForProfile(nextProfile));
    }
  }, [draftProfile, draftProfileOptions]);

  function toggleAllowedSystem(slug: string): void {
    if (isSystemDisabled(slug)) {
      return;
    }
    setAllowedSystems((previous) => {
      if (previous.includes(slug)) {
        return previous.filter((value) => value !== slug);
      }
      return [...previous, slug].sort();
    });
  }

  function isSystemDisabled(slug: string): boolean {
    const hasConfigSources = (data?.device.configSources?.length ?? 0) > 0;
    if (!hasConfigSources) {
      return false;
    }
    return !sourceScopedAllowed.has(slug);
  }

  function systemDisabledReason(slug: string): string {
    if (!isSystemDisabled(slug)) {
      return "";
    }
    return blockedReasons.get(slug)?.reason ?? "not reported by helper config";
  }

  function onDraftSystemChange(nextSystem: string): void {
    const nextProfile = profileOptionsForSystem(nextSystem)[0]?.value ?? "generic";
    setDraftSystemSlug(nextSystem);
    setDraftProfile(nextProfile);
    setDraftKind(recommendedKindForProfile(nextProfile));
  }

  function onDraftProfileChange(nextProfile: string): void {
    setDraftProfile(nextProfile);
    setDraftKind(recommendedKindForProfile(nextProfile));
  }

  function addBackendSource(): void {
    const systemSlug = draftSystemSlug.trim();
    const savePath = draftSavePath.trim();
    if (!systemSlug || !savePath) {
      setSaveError("Choose a console and enter a save folder before adding a source.");
      return;
    }
    const system = data?.systems.find((item) => item.slug === systemSlug);
    const label = draftLabel.trim() || `${system?.name ?? systemSlug} ${profileLabel(draftProfile)}`;
    const baseId = `backend-${systemSlug}-${draftProfile}`;
    const existingIds = new Set(editableSources.map((source) => source.id));
    const id = uniqueSourceId(baseId, existingIds);
    const source: DeviceConfigSource = {
      id,
      label,
      kind: draftKind,
      profile: draftProfile,
      savePaths: [savePath],
      romPaths: draftRomPath.trim() ? [draftRomPath.trim()] : [],
      recursive: true,
      systems: [systemSlug],
      createMissingSystemDirs: false,
      managed: true,
      origin: "backend"
    };
    setEditableSources((previous) => [...previous, source]);
    setSourcesDirty(true);
    setSaveError(null);
    setDraftLabel("");
    setDraftSavePath("");
    setDraftRomPath("");
  }

  function removeSource(id: string): void {
    setEditableSources((previous) => previous.filter((source) => source.id !== id));
    setSourcesDirty(true);
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
        allowedSystemSlugs: syncAll ? [] : allowedSystems,
        ...(sourcesDirty ? { configSources: editableSources } : {})
      });
      setSaveMessage("Device settings saved");
      reload();
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function onCommand(command: DeviceManageCommand): Promise<void> {
    if (!data?.device) {
      return;
    }
    setCommandKey(command);
    setSaveError(null);
    setSaveMessage(null);
    try {
      await commandDevice(data.device.id, command, "device_manage_page");
      setSaveMessage(`${commandLabel(command)} sent to ${data.device.displayName}`);
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : "Command failed");
    } finally {
      setCommandKey(null);
    }
  }

  return (
    <SectionCard title="Manage Device" subtitle="Control helper policy, folders, and sync commands from one clean loop.">
      {loading ? <LoadingState label="Loading device..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {saveError ? <ErrorState message={saveError} /> : null}
      {saveMessage ? <p className="success-state">{saveMessage}</p> : null}

      {data ? (
        <div className="device-manage-shell">
          <header className="device-manage-hero">
            <div>
              <Link className="save-detail-back" to="/app/devices">&lt; Devices</Link>
              <p className="save-detail-eyebrow">Helper control loop</p>
              <h2>{data.device.displayName}</h2>
              <p>
                {data.device.helperName || "Unknown helper"} {data.device.helperVersion ? `v${data.device.helperVersion}` : ""} · {data.device.hostname || "Unknown host"} · {data.device.lastSeenIp || "No IP"}
              </p>
            </div>
            <div className="device-manage-actions">
              <button className="btn btn-ghost" type="button" disabled={commandKey === "sync"} onClick={() => void onCommand("sync")}>
                {commandKey === "sync" ? "Sending..." : "Sync now"}
              </button>
              <button className="btn btn-ghost" type="button" disabled={commandKey === "scan"} onClick={() => void onCommand("scan")}>
                {commandKey === "scan" ? "Sending..." : "Scan folders"}
              </button>
              <button className="btn btn-ghost" type="button" disabled={commandKey === "deep_scan"} onClick={() => void onCommand("deep_scan")}>
                {commandKey === "deep_scan" ? "Sending..." : "Deep scan"}
              </button>
              <button className="btn btn-ghost" type="button" disabled={commandKey === "config_changed"} onClick={() => void onCommand("config_changed")}>
                {commandKey === "config_changed" ? "Sending..." : "Reload config"}
              </button>
            </div>
          </header>

          <DeviceManageSummary device={data.device} sources={editableSources} syncAll={syncAll} allowedSystems={allowedSystems} />

          <label className="field">
            <span>Alias</span>
            <input
              value={alias}
              onChange={(event) => setAlias(event.target.value)}
              placeholder="For example: Steam Deck"
            />
          </label>

          <label className="sync-toggle-row">
            <input type="checkbox" checked={syncAll} onChange={(event) => setSyncAll(event.target.checked)} />
            <span>
              Console sync:{" "}
              {(data.device.configSources?.length ?? 0) > 0 ? "all systems allowed by helper config" : "all supported systems"}
            </span>
          </label>

          {(data.device.configSources?.length ?? 0) > 0 ? (
            <p className="device-policy-note">
              Backend policy is scoped by the helper config. Consoles that this source cannot safely sync are disabled here and rejected by upload/download.
            </p>
          ) : null}

          <DeviceSourceEditor
            systems={data.systems}
            sources={editableSources}
            sourcesDirty={sourcesDirty}
            syncAll={syncAll}
            draftSystemSlug={draftSystemSlug}
            draftKind={draftKind}
            draftProfile={draftProfile}
            draftProfileOptions={draftProfileOptions}
            draftLabel={draftLabel}
            draftSavePath={draftSavePath}
            draftRomPath={draftRomPath}
            onSystemChange={onDraftSystemChange}
            onKindChange={setDraftKind}
            onProfileChange={onDraftProfileChange}
            onLabelChange={setDraftLabel}
            onSavePathChange={setDraftSavePath}
            onRomPathChange={setDraftRomPath}
            onAddSource={addBackendSource}
            onRemoveSource={removeSource}
          />

          {!syncAll ? (
            <SystemPolicySelector
              groups={groups}
              allowedSystems={allowedSystems}
              onToggleSystem={toggleAllowedSystem}
              isSystemDisabled={isSystemDisabled}
              systemDisabledReason={systemDisabledReason}
            />
          ) : null}

          <div className="inline-actions">
            <button className="btn btn-primary" type="button" onClick={() => void onSave()} disabled={saving}>
              {saving ? "Saving..." : "Save policy"}
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
