import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import { commandDevice, getDevice, listSaveSystems, updateDevice } from "../../services/retrosaveApi";
import type { DeviceConfigSource, DevicePolicyBlock, SaveSystem } from "../../services/types";

type SystemGroup = {
  manufacturer: string;
  systems: SaveSystem[];
};

const SOURCE_KIND_OPTIONS = [
  { value: "custom", label: "Custom" },
  { value: "retroarch", label: "RetroArch" },
  { value: "mister-fpga", label: "MiSTer FPGA" },
  { value: "steamdeck", label: "Steam Deck" },
  { value: "windows", label: "Windows" },
  { value: "openemu", label: "OpenEmu" },
  { value: "analogue-pocket", label: "Analogue Pocket" }
];

const PROFILE_OPTIONS = [
  { value: "retroarch", label: "RetroArch" },
  { value: "mister", label: "MiSTer" },
  { value: "snes9x", label: "Snes9x" },
  { value: "bsnes", label: "bsnes" },
  { value: "mesen2", label: "Mesen 2" },
  { value: "fceux", label: "FCEUX" },
  { value: "nestopia-ue", label: "Nestopia UE" },
  { value: "mgba", label: "mGBA" },
  { value: "vba-m", label: "VBA-M" },
  { value: "project64", label: "Project64" },
  { value: "mupen-family", label: "Mupen/RMG" },
  { value: "everdrive", label: "EverDrive" },
  { value: "genesis-plus-gx", label: "Genesis Plus GX" },
  { value: "picodrive", label: "PicoDrive" },
  { value: "flycast", label: "Flycast" },
  { value: "redream", label: "Redream" },
  { value: "generic", label: "Generic" }
];

const SYSTEM_PROFILE_OPTIONS: Record<string, Array<{ value: string; label: string }>> = {
  snes: [
    { value: "snes9x", label: "Snes9x" },
    { value: "bsnes", label: "bsnes" },
    { value: "retroarch-snes9x", label: "RetroArch Snes9x" },
    { value: "mesen2", label: "Mesen 2" },
    { value: "higan", label: "higan" }
  ],
  nes: [
    { value: "mesen2", label: "Mesen 2" },
    { value: "fceux", label: "FCEUX" },
    { value: "nestopia-ue", label: "Nestopia UE" },
    { value: "punes", label: "puNES" },
    { value: "retroarch-fceumm", label: "RetroArch FCEUmm" }
  ],
  gba: [
    { value: "mgba", label: "mGBA" },
    { value: "vba-m", label: "VBA-M" },
    { value: "nocashgba", label: "No$GBA" },
    { value: "skyemu", label: "SkyEmu" }
  ],
  n64: [
    { value: "mister", label: "MiSTer" },
    { value: "retroarch", label: "RetroArch" },
    { value: "project64", label: "Project64" },
    { value: "mupen-family", label: "Mupen/RMG" },
    { value: "everdrive", label: "EverDrive" }
  ],
  genesis: [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "picodrive", label: "PicoDrive" },
    { value: "blastem", label: "BlastEm" }
  ],
  "sega-cd": [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "picodrive", label: "PicoDrive" },
    { value: "retroarch-genesis-plus-gx", label: "RetroArch Genesis Plus GX" }
  ],
  "sega-32x": [
    { value: "picodrive", label: "PicoDrive" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "retroarch-picodrive", label: "RetroArch PicoDrive" }
  ],
  "master-system": [
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "gearsystem", label: "Gearsystem" },
    { value: "emulicious", label: "Emulicious" },
    { value: "meka", label: "MEKA" }
  ],
  "game-gear": [
    { value: "gearsystem", label: "Gearsystem" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" },
    { value: "emulicious", label: "Emulicious" }
  ],
  "pc-engine": [
    { value: "mister", label: "MiSTer" },
    { value: "mednafen", label: "Mednafen" },
    { value: "retroarch-beetle-pce", label: "RetroArch Beetle PCE" },
    { value: "mesen2", label: "Mesen 2" }
  ],
  "atari-lynx": [
    { value: "handy", label: "Handy" },
    { value: "mednafen", label: "Mednafen" },
    { value: "retroarch-handy", label: "RetroArch Handy" }
  ],
  wonderswan: [
    { value: "mednafen", label: "Mednafen" },
    { value: "ares", label: "ares" },
    { value: "retroarch-beetle-wswan", label: "RetroArch Beetle WonderSwan" }
  ],
  "sg-1000": [
    { value: "emulicious", label: "Emulicious" },
    { value: "gearsystem", label: "Gearsystem" },
    { value: "genesis-plus-gx", label: "Genesis Plus GX" }
  ],
  colecovision: [
    { value: "blue-msx", label: "blueMSX" },
    { value: "gearcoleco", label: "Gearcoleco" },
    { value: "mame", label: "MAME" }
  ],
  "atari-jaguar": [
    { value: "bigpemu", label: "BigPEmu" },
    { value: "virtual-jaguar", label: "Virtual Jaguar" },
    { value: "retroarch-virtual-jaguar", label: "RetroArch Virtual Jaguar" }
  ],
  "3do": [
    { value: "opera", label: "Opera" },
    { value: "phoenix", label: "Phoenix" },
    { value: "4do", label: "4DO" }
  ],
  dreamcast: [
    { value: "flycast", label: "Flycast" },
    { value: "redream", label: "Redream" },
    { value: "mister", label: "MiSTer" },
    { value: "retroarch-flycast", label: "RetroArch Flycast" }
  ],
  saturn: [
    { value: "mister", label: "MiSTer" },
    { value: "mednafen", label: "Mednafen" },
    { value: "yabasanshiro", label: "Yaba Sanshiro" },
    { value: "yabause", label: "Yabause" }
  ],
  psx: [
    { value: "mister", label: "MiSTer" },
    { value: "retroarch", label: "RetroArch" }
  ],
  ps2: [{ value: "pcsx2", label: "PCSX2" }]
};

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

  async function onCommand(command: "sync" | "scan" | "deep_scan"): Promise<void> {
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
    <SectionCard title="Manage Device" subtitle="Rename the helper, choose allowed consoles, and add backend-managed sync sources.">
      {loading ? <LoadingState label="Loading device..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {saveError ? <ErrorState message={saveError} /> : null}
      {saveMessage ? <p className="success-state">{saveMessage}</p> : null}

      {data ? (
        <div className="stack">
          <p>
            <strong>Device:</strong> {data.device.displayName} ({data.device.deviceType} · {data.device.fingerprint})
          </p>
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
          </div>

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

          <section className="device-source-editor">
            <div className="device-source-editor__header">
              <div>
                <h3>Add console source</h3>
                <p>Add folders the always-on helper should include in its next backend-managed config policy.</p>
              </div>
              {sourcesDirty ? <span>Unsaved changes</span> : null}
            </div>

            <div className="device-source-form">
              <label className="field">
                <span>Console</span>
                <select
                  value={draftSystemSlug}
                  onChange={(event) => {
                    const nextSystem = event.target.value;
                    const nextProfile = profileOptionsForSystem(nextSystem)[0]?.value ?? "generic";
                    setDraftSystemSlug(nextSystem);
                    setDraftProfile(nextProfile);
                    setDraftKind(recommendedKindForProfile(nextProfile));
                  }}
                >
                  {data.systems
                    .filter((system) => Boolean(system.slug))
                    .map((system) => (
                      <option key={system.slug} value={system.slug}>
                        {system.name}
                      </option>
                    ))}
                </select>
              </label>
              <label className="field">
                <span>Runtime kind</span>
                <select value={draftKind} onChange={(event) => setDraftKind(event.target.value)}>
                  {SOURCE_KIND_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span>Profile</span>
                <select
                  value={draftProfile}
                  onChange={(event) => {
                    setDraftProfile(event.target.value);
                    setDraftKind(recommendedKindForProfile(event.target.value));
                  }}
                >
                  {draftProfileOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span>Label</span>
                <input value={draftLabel} onChange={(event) => setDraftLabel(event.target.value)} placeholder="Optional display name" />
              </label>
              <label className="field device-source-form__wide">
                <span>Save folder</span>
                <input value={draftSavePath} onChange={(event) => setDraftSavePath(event.target.value)} placeholder="/media/snes9x/saves" />
              </label>
              <label className="field device-source-form__wide">
                <span>ROM folder</span>
                <input value={draftRomPath} onChange={(event) => setDraftRomPath(event.target.value)} placeholder="/media/snes9x/roms (optional)" />
              </label>
            </div>
            <button className="btn btn-ghost" type="button" onClick={addBackendSource}>
              Add console
            </button>

            <div className="device-source-list">
              {editableSources.length === 0 ? <p>No helper config sources reported yet.</p> : null}
              {editableSources.map((source) => (
                <article key={source.id} className="device-source-row">
                  <div>
                    <strong>{source.label || source.id}</strong>
                    <p>
                      {[source.kind, source.profile, source.origin].filter(Boolean).join(" / ") || "Unknown source"}
                    </p>
                    <small>
                      {(source.systems ?? []).join(", ") || "No systems"} · {formatSourcePaths(source.savePaths ?? (source.savePath ? [source.savePath] : []))}
                    </small>
                  </div>
                  <button className="btn btn-ghost btn-danger" type="button" onClick={() => removeSource(source.id)}>
                    Remove
                  </button>
                </article>
              ))}
            </div>
            <DevicePolicyPreview syncAll={syncAll} sources={editableSources} />
          </section>

          {!syncAll ? (
            <div className="stack compact">
              {groups.map((group) => (
                <details key={group.manufacturer} className="device-group" open>
                  <summary>
                    <strong>{group.manufacturer}</strong>
                    <span>{group.systems.length} systems</span>
                  </summary>
                  <div className="device-group__list">
                    {group.systems.map((system) => {
                      const slug = system.slug ?? "";
                      const disabled = isSystemDisabled(slug);
                      const reason = systemDisabledReason(slug);
                      return (
                        <label key={slug} className="sync-option-row">
                          <input
                            type="checkbox"
                            checked={allowedSystems.includes(slug)}
                            disabled={disabled}
                            onChange={() => toggleAllowedSystem(slug)}
                          />
                          <span>
                            {system.name}
                            {disabled ? <small>Blocked: {reason}</small> : null}
                          </span>
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

function cloneSource(source: DeviceConfigSource): DeviceConfigSource {
  return {
    ...source,
    savePaths: source.savePaths ? [...source.savePaths] : undefined,
    romPaths: source.romPaths ? [...source.romPaths] : undefined,
    systems: source.systems ? [...source.systems] : undefined,
    unsupportedSystemSlugs: source.unsupportedSystemSlugs ? [...source.unsupportedSystemSlugs] : undefined
  };
}

function uniqueSourceId(baseId: string, existingIds: Set<string>): string {
  let id = baseId;
  let suffix = 2;
  while (existingIds.has(id)) {
    id = `${baseId}-${suffix}`;
    suffix += 1;
  }
  return id;
}

function profileLabel(profile: string): string {
  return PROFILE_OPTIONS.find((option) => option.value === profile)?.label
    ?? Object.values(SYSTEM_PROFILE_OPTIONS).flat().find((option) => option.value === profile)?.label
    ?? profile;
}

function formatSourcePaths(paths: string[]): string {
  if (paths.length === 0) {
    return "no save path";
  }
  if (paths.length === 1) {
    return paths[0];
  }
  return `${paths[0]} + ${paths.length - 1} more`;
}

function profileOptionsForSystem(systemSlug: string): Array<{ value: string; label: string }> {
  const scoped = SYSTEM_PROFILE_OPTIONS[systemSlug.trim()];
  if (scoped && scoped.length > 0) {
    return scoped;
  }
  return PROFILE_OPTIONS;
}

function recommendedKindForProfile(profile: string): string {
  if (profile === "mister" || profile === "everdrive") {
    return "mister-fpga";
  }
  if (profile.startsWith("retroarch")) {
    return "retroarch";
  }
  if (["snes9x", "bsnes", "mesen2", "fceux", "nestopia-ue", "mgba", "vba-m", "project64", "mupen-family", "pcsx2"].includes(profile)) {
    return "custom";
  }
  return "custom";
}

function commandLabel(command: "sync" | "scan" | "deep_scan"): string {
  switch (command) {
    case "sync":
      return "Sync";
    case "scan":
      return "Scan";
    case "deep_scan":
      return "Deep scan";
    default:
      return command;
  }
}

function DevicePolicyPreview({ syncAll, sources }: { syncAll: boolean; sources: DeviceConfigSource[] }): JSX.Element {
  const systems = Array.from(new Set(sources.flatMap((source) => source.systems ?? []))).sort();
  const managed = sources.filter((source) => source.managed).length;
  return (
    <aside className="device-policy-preview" aria-label="Policy preview">
      <div>
        <span>Policy preview</span>
        <strong>{syncAll ? "Sync all allowed systems" : `${systems.length} manually selected`}</strong>
      </div>
      <p>
        {sources.length} source profiles, {managed} backend-managed. {systems.length > 0 ? `Consoles: ${systems.join(", ")}.` : "No backend console sources yet."}
      </p>
    </aside>
  );
}
