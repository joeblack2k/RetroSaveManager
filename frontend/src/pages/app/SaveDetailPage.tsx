import { useCallback, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getSaveCheats, getSaveHistory, rollbackSave } from "../../services/retrosaveApi";
import type { MemoryCardEntry, SaveCheatEditorState, SaveCheatField, SaveDownloadProfile, SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";
import { buildSaveInsight, type SaveInsightModel } from "../../utils/saveInsights";
import { buildSaveDetailsHref, buildSaveDownloadHref } from "../../utils/saveRows";

type DownloadModalState = {
  title: string;
  request: { saveId: string; psLogicalKey?: string; revisionId?: string };
  profiles: SaveDownloadProfile[];
};

type DetailMetric = {
  label: string;
  value: string;
};

export function SaveDetailPage(): JSX.Element {
  const params = useParams<{ saveId: string }>();
  const [searchParams] = useSearchParams();
  const saveId = params.saveId ?? "";
  const psLogicalKey = (searchParams.get("psLogicalKey") || "").trim();
  const [rollbackError, setRollbackError] = useState<string | null>(null);
  const [rollbackMessage, setRollbackMessage] = useState<string | null>(null);
  const [rollbackingId, setRollbackingId] = useState<string | null>(null);
  const [downloadState, setDownloadState] = useState<DownloadModalState | null>(null);

  const loader = useCallback(() => getSaveHistory({ saveId, psLogicalKey: psLogicalKey || undefined }), [saveId, psLogicalKey]);
  const { loading, error, data, reload } = useAsyncData(loader, [saveId, psLogicalKey]);

  const versions = data?.versions ?? [];
  const latest = versions.length > 0 ? versions[0] : null;
  const fallbackSummary = useMemo(() => buildSummaryFromVersions(versions), [versions]);
  const memoryCard = useMemo(
    () => versions.find((version) => version.memoryCard?.entries && version.memoryCard.entries.length > 0)?.memoryCard || latest?.memoryCard || null,
    [latest?.memoryCard, versions]
  );
  const playStationEntries = useMemo(() => uniqueMemoryCardEntries(memoryCard?.entries), [memoryCard?.entries]);
  const systemSlug = normalizeSystemSlug(data?.systemSlug || latest?.systemSlug || latest?.game.system?.slug || "");
  const isPlayStationSystem = systemSlug === "psx" || systemSlug === "ps2";
  const effectiveLogicalKey = useMemo(() => {
    if (psLogicalKey) {
      return psLogicalKey;
    }
    if (!isPlayStationSystem || playStationEntries.length !== 1) {
      return "";
    }
    return (playStationEntries[0]?.logicalKey || "").trim();
  }, [isPlayStationSystem, playStationEntries, psLogicalKey]);
  const logicalEntry = useMemo(() => {
    if (!effectiveLogicalKey) {
      return null;
    }
    return playStationEntries.find((entry) => (entry.logicalKey || "").trim() === effectiveLogicalKey) || playStationEntries[0] || null;
  }, [effectiveLogicalKey, playStationEntries]);
  const showPlayStationSelector = isPlayStationSystem && !effectiveLogicalKey && playStationEntries.length > 0;

  const rawDisplayTitle = (data?.displayTitle || data?.summary?.displayTitle || latest?.displayTitle || latest?.game.displayTitle || latest?.game.name || "Unknown game").trim();
  const displayTitle = useMemo(() => {
    if (logicalEntry?.title) {
      return logicalEntry.title.trim() || "Unknown game";
    }
    if (showPlayStationSelector) {
      return "Choose a PlayStation save";
    }
    return rawDisplayTitle || "Unknown game";
  }, [logicalEntry?.title, rawDisplayTitle, showPlayStationSelector]);
  const systemName = data?.summary?.system?.name || latest?.game.system?.name || "Unknown console";
  const regionCode = normalizeRegionCode((data?.summary?.regionCode || latest?.regionCode || latest?.game.regionCode || logicalEntry?.regionCode || "UNKNOWN").toString());
  const languageCodes = mergeLanguageCodes(data?.summary?.languageCodes, latest?.languageCodes, latest?.game.languageCodes, fallbackSummary.languageCodes);
  const saveCount = showPlayStationSelector ? playStationEntries.length : data?.summary?.saveCount || fallbackSummary.saveCount;
  const totalSizeBytes = showPlayStationSelector
    ? playStationEntries.reduce((sum, entry) => sum + (entry.totalSizeBytes || entry.sizeBytes || 0), 0)
    : data?.summary?.totalSizeBytes || fallbackSummary.totalSizeBytes;
  const latestVersion = showPlayStationSelector ? 0 : data?.summary?.latestVersion || fallbackSummary.latestVersion;
  const latestCreatedAt = data?.summary?.latestCreatedAt || fallbackSummary.latestCreatedAt;
  const currentSizeBytes = logicalEntry?.sizeBytes || latest?.fileSize || 0;
  const saveInsight = useMemo(() => buildSaveInsight(latest), [latest]);
  const hasCheats = Boolean(latest?.cheats?.supported && (latest.cheats.availableCount || 0) > 0);
  const cheatSaveId = latest && hasCheats && !showPlayStationSelector ? latest.id : "";
  const cheatLoader = useCallback(async () => {
    if (!cheatSaveId) {
      return null;
    }
    const response = await getSaveCheats(cheatSaveId);
    return response.cheats?.supported ? response.cheats : null;
  }, [cheatSaveId]);
  const { loading: cheatLoading, error: cheatError, data: cheatData } = useAsyncData<SaveCheatEditorState | null>(cheatLoader, [cheatSaveId]);
  const parserBadge = saveInsight?.parserLevel || (latest?.inspection ? "Verified" : "Protected");
  const currentDownloadRequest = latest
    ? {
        saveId: effectiveLogicalKey ? saveId : latest.id,
        psLogicalKey: effectiveLogicalKey || undefined,
        revisionId: effectiveLogicalKey ? latest.id : undefined
      }
    : null;

  const heroMetrics: DetailMetric[] = [
    { label: "Console", value: systemName },
    { label: "Region", value: showPlayStationSelector ? "Multiple" : `${regionToFlagEmoji(regionCode)} ${regionCode}` },
    { label: showPlayStationSelector ? "Available saves" : "Versions", value: String(saveCount) },
    { label: "Current size", value: formatBytes(currentSizeBytes || totalSizeBytes) },
    { label: "Updated", value: formatDate(latestCreatedAt) },
    { label: "Decoder", value: parserBadge }
  ];

  async function handleRollback(target: SaveSummary): Promise<void> {
    const confirmed = window.confirm(`Rollback to version ${target.version} of ${displayTitle}?\n\nA new latest version will be created as the promoted sync copy.`);
    if (!confirmed) {
      return;
    }

    setRollbackError(null);
    setRollbackMessage(null);
    setRollbackingId(target.id);
    try {
      const response = await rollbackSave(
        effectiveLogicalKey
          ? { saveId, psLogicalKey: effectiveLogicalKey, revisionId: target.id }
          : { saveId: target.id }
      );
      setRollbackMessage(`Rollback complete. New current version: v${response.save.version}`);
      await reload();
    } catch (err: unknown) {
      setRollbackError(err instanceof Error ? err.message : "Rollback failed.");
    } finally {
      setRollbackingId(null);
    }
  }

  function openDownloadModal(title: string, request: { saveId: string; psLogicalKey?: string; revisionId?: string }, profiles: SaveDownloadProfile[] | undefined): void {
    const normalized = profiles && profiles.length > 0 ? profiles : [{ id: "original", label: "Original file" }];
    setDownloadState({ title, request, profiles: normalized });
  }

  function closeDownloadModal(): void {
    setDownloadState(null);
  }

  return (
    <SectionCard title="Save Details" subtitle="A clean readout of the current sync save, parser-backed gameplay facts, and version history.">
      {loading ? <LoadingState label="Loading save history..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {!loading && !error && versions.length === 0 ? <ErrorState message="No versions found for this save." /> : null}

      {!loading && !error && versions.length > 0 ? (
        <div className="save-detail-shell">
          <header className="save-detail-hero">
            <div className="save-detail-hero__main">
              <Link className="save-detail-back" to="/app/my-games">&lt; My Saves</Link>
              <p className="save-detail-eyebrow">{systemName} / current sync save</p>
              <h2>{displayTitle}</h2>
              <p className="save-detail-subtitle">
                {showPlayStationSelector
                  ? "This memory-card upload contains multiple game saves. Select one to inspect its own history."
                  : `Version ${latestVersion} is leading for sync. Last updated ${formatDate(latestCreatedAt)}.`}
              </p>
              <div className="save-detail-metrics" aria-label="Save summary">
                {heroMetrics.map((metric) => (
                  <div className="save-detail-metric" key={metric.label}>
                    <span>{metric.label}</span>
                    <strong>{metric.value}</strong>
                  </div>
                ))}
              </div>
            </div>
            <div className="save-detail-actions">
              {hasCheats ? <span className="save-detail-tag save-detail-tag--cheats">Cheats available</span> : null}
              {saveInsight?.parserLevel ? <span className="save-detail-tag">{saveInsight.parserLevel} verified</span> : null}
              {currentDownloadRequest ? (
                <button
                  className="save-detail-primary-btn"
                  type="button"
                  onClick={() => openDownloadModal(displayTitle, currentDownloadRequest, latest?.downloadProfiles)}
                >
                  Download current
                </button>
              ) : null}
            </div>
          </header>

          {rollbackError ? <p className="error-state">{rollbackError}</p> : null}
          {rollbackMessage ? <p className="success-state">{rollbackMessage}</p> : null}

          {showPlayStationSelector ? (
            <PlayStationSavePicker entries={playStationEntries} saveId={saveId} latest={latest} openDownloadModal={openDownloadModal} />
          ) : (
            <>
              <DecodedSavePanel
                insight={saveInsight}
                systemName={systemName}
                cheatData={cheatData}
                cheatLoading={Boolean(cheatSaveId) && cheatLoading}
                cheatError={Boolean(cheatSaveId) ? cheatError : null}
              />
              {logicalEntry ? <LogicalSavePanel entry={logicalEntry} latest={latest} /> : null}
              <TechnicalDetailsPanel latest={latest} insight={saveInsight} languageCodes={languageCodes} />
              <VersionHistoryTable
                versions={versions}
                displayTitle={displayTitle}
                effectiveLogicalKey={effectiveLogicalKey}
                saveId={saveId}
                rollbackingId={rollbackingId}
                handleRollback={handleRollback}
                openDownloadModal={openDownloadModal}
              />
            </>
          )}
        </div>
      ) : null}

      {downloadState ? (
        <div className="treegrid-modal-backdrop" role="presentation" onClick={closeDownloadModal}>
          <section
            className="treegrid-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="save-detail-download-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="treegrid-modal__header">
              <div>
                <h2 id="save-detail-download-title">Download Save</h2>
                <p>{downloadState.title}</p>
              </div>
              <button className="treegrid-modal__close" type="button" onClick={closeDownloadModal} aria-label="Close download options">
                Close
              </button>
            </header>

            <div className="treegrid-modal__body">
              <table className="treegrid-modal-table">
                <thead>
                  <tr>
                    <th>Profile</th>
                    <th>Extension</th>
                    <th>Notes</th>
                    <th>Download</th>
                  </tr>
                </thead>
                <tbody>
                  {downloadState.profiles.map((profile) => (
                    <tr key={profile.id}>
                      <td>{profile.label}</td>
                      <td>{profile.targetExtension || "-"}</td>
                      <td>{profile.note || "-"}</td>
                      <td>
                        <a
                          className="saves-action-link"
                          href={buildSaveDownloadHref(downloadState.request, profile.id !== "original" ? profile.id : undefined)}
                        >
                          Download
                        </a>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      ) : null}
    </SectionCard>
  );
}

function DecodedSavePanel({
  insight,
  systemName,
  cheatData,
  cheatLoading,
  cheatError
}: {
  insight: SaveInsightModel | null;
  systemName: string;
  cheatData: SaveCheatEditorState | null;
  cheatLoading: boolean;
  cheatError: string | null;
}): JSX.Element {
  const parserGameplayRows = insight?.rows.filter((row) => row.kind === "gameplay") ?? [];
  const cheatRows = buildCheatFactRows(cheatData);
  const gameplayRows = mergeDetailRows(parserGameplayRows, cheatRows).slice(0, 18);
  if (gameplayRows.length > 0) {
    const cheatOnly = parserGameplayRows.length === 0 && cheatRows.length > 0;
    return (
      <section className="save-detail-panel" aria-labelledby="save-detail-decoder-title">
        <div className="save-detail-panel__header">
          <div>
            <p className="save-detail-eyebrow">{cheatOnly ? "Editable save values" : "Decoded gameplay"}</p>
            <h3 id="save-detail-decoder-title">{cheatOnly ? "Cheat-backed save values" : insight?.title || "Gameplay facts"}</h3>
            <p>{cheatOnly ? "Current values read through the same safe parser-backed editor used by cheats." : insight?.subtitle || "Parser-backed facts from the current save."}</p>
          </div>
          <span className="save-detail-tag save-detail-tag--good">{cheatOnly ? cheatData?.editorId || "cheat parser" : insight?.parserId || "parser active"}</span>
        </div>
        <div className="save-detail-gameplay-grid">
          {gameplayRows.map((row) => (
            <div className="save-detail-gameplay-card" key={row.label}>
              <span>{row.label}</span>
              <strong>{row.value}</strong>
            </div>
          ))}
        </div>
        {cheatError ? <p className="error-state">Could not read cheat-backed values: {cheatError}</p> : null}
        {insight?.warnings.length ? (
          <div className="save-detail-note-line">
            {insight.warnings.slice(0, 2).map((warning) => (
              <span key={warning}>{warning}</span>
            ))}
          </div>
        ) : null}
      </section>
    );
  }

  const verifiedRows = insight?.rows.filter((row) => row.kind !== "gameplay").slice(0, 8) ?? [];
  if (verifiedRows.length > 0 || insight) {
    return (
      <section className="save-detail-panel" aria-labelledby="save-detail-decoder-title">
        <div className="save-detail-panel__header">
          <div>
            <p className="save-detail-eyebrow">Verified save facts</p>
            <h3 id="save-detail-decoder-title">{insight?.title || "Save verified"}</h3>
            <p>{cheatLoading ? "Checking parser-backed cheat values..." : insight?.subtitle || "Verified backend metadata from the current save."}</p>
          </div>
          <span className="save-detail-tag">{insight?.parserId || "verified"}</span>
        </div>
        {verifiedRows.length > 0 ? (
          <div className="save-detail-gameplay-grid">
            {verifiedRows.map((row) => (
              <div className="save-detail-gameplay-card save-detail-gameplay-card--verified" key={row.label}>
                <span>{row.label}</span>
                <strong>{row.value}</strong>
              </div>
            ))}
          </div>
        ) : null}
        {cheatError ? <p className="error-state">Could not read cheat-backed values: {cheatError}</p> : null}
        {insight?.warnings.length ? (
          <div className="save-detail-note-line">
            {insight.warnings.slice(0, 2).map((warning) => (
              <span key={warning}>{warning}</span>
            ))}
          </div>
        ) : null}
      </section>
    );
  }

  if (!cheatLoading) {
    return (
      <section className="save-detail-panel save-detail-decoder-empty" aria-labelledby="save-detail-decoder-title">
        <div>
          <p className="save-detail-eyebrow">Save protected</p>
          <h3 id="save-detail-decoder-title">No decoder attached yet</h3>
          <p>
            This save is still protected and versioned. Add a parser-backed Game Support Module for {systemName} to show lives, world, stage, inventory, and other fun details here automatically.
          </p>
        </div>
        <span className="save-detail-tag">Waiting for parser</span>
      </section>
    );
  }

  return (
    <section className="save-detail-panel save-detail-decoder-empty" aria-labelledby="save-detail-decoder-title">
      <div>
        <p className="save-detail-eyebrow">Gameplay decoder</p>
        <h3 id="save-detail-decoder-title">Reading save values...</h3>
        <p>Checking parser-backed cheat values for this save.</p>
      </div>
      <span className="save-detail-tag">Loading</span>
    </section>
  );
}

function buildCheatFactRows(cheats: SaveCheatEditorState | null): DetailMetric[] {
  if (!cheats?.supported) {
    return [];
  }
  const { values, slotLabel } = selectCheatValues(cheats);
  const rows: DetailMetric[] = [];
  if (slotLabel) {
    rows.push({ label: "Save file", value: slotLabel });
  }

  const fields = (cheats.sections ?? []).flatMap((section) => section.fields ?? []);
  const seenKeys = new Set<string>();
  for (const field of fields) {
    const key = field.ref || field.id;
    if (!key || seenKeys.has(key) || !Object.prototype.hasOwnProperty.call(values, key)) {
      continue;
    }
    const formatted = formatCheatValue(field, values[key]);
    if (!formatted) {
      continue;
    }
    seenKeys.add(key);
    rows.push({ label: field.label || humanizeDetailLabel(key), value: formatted });
    if (rows.length >= 18) {
      return rows;
    }
  }

  for (const [key, value] of Object.entries(values)) {
    if (seenKeys.has(key) || rows.length >= 18) {
      continue;
    }
    const formatted = formatBasicDetailValue(value);
    if (!formatted) {
      continue;
    }
    rows.push({ label: humanizeDetailLabel(key), value: formatted });
  }

  return rows;
}

function selectCheatValues(cheats: SaveCheatEditorState): { values: Record<string, unknown>; slotLabel: string } {
  if (cheats.values && Object.keys(cheats.values).length > 0) {
    return { values: cheats.values, slotLabel: "" };
  }
  const slotValues = cheats.slotValues ?? {};
  const selectorOptions = cheats.selector?.options ?? [];
  for (const option of selectorOptions) {
    const values = slotValues[option.id];
    if (values && Object.keys(values).length > 0) {
      return { values, slotLabel: option.label || option.id };
    }
  }
  const firstSlot = Object.keys(slotValues)[0];
  if (firstSlot) {
    return { values: slotValues[firstSlot] ?? {}, slotLabel: firstSlot };
  }
  return { values: {}, slotLabel: "" };
}

function mergeDetailRows(primary: DetailMetric[], secondary: DetailMetric[]): DetailMetric[] {
  const rows: DetailMetric[] = [];
  const seen = new Set<string>();
  for (const row of [...primary, ...secondary]) {
    const key = row.label.trim().toLowerCase();
    if (!row.value || seen.has(key)) {
      continue;
    }
    seen.add(key);
    rows.push(row);
  }
  return rows;
}

function formatCheatValue(field: SaveCheatField, value: unknown): string {
  if (field.type === "boolean" && typeof value === "boolean") {
    return value ? "Enabled" : "Disabled";
  }
  if (field.type === "enum" && typeof value === "string") {
    return field.options?.find((option) => option.id === value)?.label || humanizeDetailLabel(value);
  }
  if (field.type === "bitmask" && Array.isArray(value)) {
    const labels = value
      .map((item) => String(item))
      .map((item) => field.bits?.find((bit) => bit.id === item)?.label || item)
      .filter(Boolean);
    return labels.join(", ");
  }
  return formatBasicDetailValue(value);
}

function formatBasicDetailValue(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "";
  }
  if (typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? String(value) : "";
  }
  if (typeof value === "string") {
    return value.trim();
  }
  if (Array.isArray(value)) {
    return value.map((item) => formatBasicDetailValue(item)).filter(Boolean).join(", ");
  }
  return "";
}

function humanizeDetailLabel(value: string): string {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim()
    .replace(/\s+/g, " ")
    .replace(/^./, (char) => char.toUpperCase());
}

function TechnicalDetailsPanel({ latest, insight, languageCodes }: { latest: SaveSummary | null; insight: SaveInsightModel | null; languageCodes: string[] }): JSX.Element | null {
  if (!latest && !insight) {
    return null;
  }
  const technicalRows = insight?.rows.filter((row) => row.kind !== "gameplay") ?? [];
  const rows: DetailMetric[] = [
    { label: "Filename", value: latest?.filename || "-" },
    { label: "Format", value: latest?.format || "-" },
    { label: "SHA256", value: latest?.sha256 || "-" },
    { label: "Languages", value: languageCodes.length > 0 ? languageCodes.join(", ") : "-" },
    { label: "Source profile", value: latest?.sourceArtifactProfile || "-" },
    { label: "Runtime profile", value: latest?.runtimeProfile || "-" }
  ];
  for (const row of technicalRows) {
    rows.push({ label: row.label, value: row.value });
  }

  return (
    <details className="save-detail-technical">
      <summary>Verified technical data</summary>
      <div className="save-detail-technical-grid">
        {rows.filter((row) => row.value && row.value !== "-").map((row) => (
          <div className="save-detail-tech-row" key={`${row.label}:${row.value}`}>
            <span>{row.label}</span>
            <strong>{row.value}</strong>
          </div>
        ))}
      </div>
      {insight?.evidence.length ? (
        <div className="save-detail-evidence">
          <span>Parser evidence</span>
          <ul>
            {insight.evidence.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </details>
  );
}

function LogicalSavePanel({ entry, latest }: { entry: MemoryCardEntry; latest: SaveSummary | null }): JSX.Element {
  return (
    <section className="save-detail-panel save-detail-logical" aria-label="Logical save">
      {entry.iconDataUrl ? <img className="memory-card-entry-preview" src={entry.iconDataUrl} alt={`${entry.title} icon`} loading="lazy" /> : null}
      <div>
        <p className="save-detail-eyebrow">Logical save</p>
        <h3>{entry.title}</h3>
        <p>
          {entry.productCode ? `${entry.productCode} / ` : ""}
          {entry.directoryName || "PlayStation entry"}
        </p>
      </div>
      <div className="save-detail-logical__stats">
        {entry.slot > 0 ? <span>Slot {entry.slot}</span> : null}
        {entry.blocks > 0 ? <span>{entry.blocks} blocks</span> : null}
        <span>{formatBytes(entry.sizeBytes || latest?.fileSize || 0)}</span>
      </div>
    </section>
  );
}

function PlayStationSavePicker({
  entries,
  saveId,
  latest,
  openDownloadModal
}: {
  entries: MemoryCardEntry[];
  saveId: string;
  latest: SaveSummary | null;
  openDownloadModal: (title: string, request: { saveId: string; psLogicalKey?: string; revisionId?: string }, profiles: SaveDownloadProfile[] | undefined) => void;
}): JSX.Element {
  return (
    <section className="save-detail-panel" aria-labelledby="save-detail-picker-title">
      <div className="save-detail-panel__header">
        <div>
          <p className="save-detail-eyebrow">Memory card entries</p>
          <h3 id="save-detail-picker-title">Select a save</h3>
          <p>Each entry has its own details, version history, and download options.</p>
        </div>
      </div>
      <div className="save-detail-table-wrap">
        <table className="save-detail-table">
          <thead>
            <tr>
              <th>Save</th>
              <th>Region</th>
              <th>Size</th>
              <th>Details</th>
              <th>Download</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, index) => {
              const entryLogicalKey = (entry.logicalKey || "").trim();
              const canOpen = entryLogicalKey !== "";
              return (
                <tr key={`${entryLogicalKey || entry.productCode || entry.directoryName || entry.title}-${index}`}>
                  <td>
                    <div className="save-detail-entry-cell">
                      {entry.iconDataUrl ? (
                        <img className="memory-card-entry-preview" src={entry.iconDataUrl} alt={`${entry.title} icon`} loading="lazy" />
                      ) : (
                        <div className="memory-card-entry-preview memory-card-entry-preview--empty" aria-hidden="true" />
                      )}
                      <div>
                        <strong>{entry.title}</strong>
                        <span>{entry.productCode || entry.directoryName || "Memory card save"}</span>
                      </div>
                    </div>
                  </td>
                  <td>{regionToFlagEmoji((entry.regionCode || "UNKNOWN").toString())} {normalizeRegionCode((entry.regionCode || "UNKNOWN").toString())}</td>
                  <td>{formatBytes(entry.totalSizeBytes || entry.sizeBytes || 0)}</td>
                  <td>
                    {canOpen ? (
                      <Link className="saves-action-link" to={buildSaveDetailsHref({ primarySaveID: saveId, psLogicalKey: entryLogicalKey })}>
                        Details
                      </Link>
                    ) : (
                      <span>-</span>
                    )}
                  </td>
                  <td>
                    {canOpen ? (
                      <button
                        className="saves-action-btn"
                        type="button"
                        onClick={() => openDownloadModal(entry.title, { saveId, psLogicalKey: entryLogicalKey }, latest?.downloadProfiles)}
                      >
                        Download
                      </button>
                    ) : (
                      <span>-</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function VersionHistoryTable({
  versions,
  displayTitle,
  effectiveLogicalKey,
  saveId,
  rollbackingId,
  handleRollback,
  openDownloadModal
}: {
  versions: SaveSummary[];
  displayTitle: string;
  effectiveLogicalKey: string;
  saveId: string;
  rollbackingId: string | null;
  handleRollback: (target: SaveSummary) => Promise<void>;
  openDownloadModal: (title: string, request: { saveId: string; psLogicalKey?: string; revisionId?: string }, profiles: SaveDownloadProfile[] | undefined) => void;
}): JSX.Element {
  return (
    <section className="save-detail-panel" aria-labelledby="save-detail-history-title">
      <div className="save-detail-panel__header">
        <div>
          <p className="save-detail-eyebrow">History</p>
          <h3 id="save-detail-history-title">Sync versions</h3>
          <p>Only meaningful controls are shown here. Full hashes and file internals live under technical data.</p>
        </div>
      </div>
      <div className="save-detail-table-wrap">
        <table className="save-detail-table">
          <thead>
            <tr>
              <th>Version</th>
              <th>Date</th>
              <th>Size</th>
              <th>Status</th>
              <th>Download</th>
              <th>Rollback</th>
            </tr>
          </thead>
          <tbody>
            {versions.map((version, index) => {
              const isLatest = index === 0;
              const isBusy = rollbackingId === version.id;
              return (
                <tr key={version.id}>
                  <td>v{version.version}</td>
                  <td>{formatDate(version.createdAt)}</td>
                  <td>{formatBytes(version.fileSize)}</td>
                  <td>{isLatest ? <span className="save-detail-status">Current sync</span> : <span className="save-detail-status save-detail-status--old">History</span>}</td>
                  <td>
                    <button
                      className="saves-action-btn"
                      type="button"
                      onClick={() =>
                        openDownloadModal(displayTitle, {
                          saveId: effectiveLogicalKey ? saveId : version.id,
                          psLogicalKey: effectiveLogicalKey || undefined,
                          revisionId: effectiveLogicalKey ? version.id : undefined
                        }, version.downloadProfiles)
                      }
                    >
                      Download
                    </button>
                  </td>
                  <td>
                    <button className="saves-action-btn" type="button" disabled={isLatest || isBusy} onClick={() => void handleRollback(version)}>
                      {isLatest ? "Current" : isBusy ? "..." : "Rollback"}
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function uniqueMemoryCardEntries(entries: MemoryCardEntry[] | undefined): MemoryCardEntry[] {
  if (!entries || entries.length === 0) {
    return [];
  }
  const out: MemoryCardEntry[] = [];
  const seen = new Set<string>();
  for (const entry of entries) {
    const logicalKey = (entry.logicalKey || "").trim();
    const identity = logicalKey || `${entry.productCode || entry.directoryName || entry.title}:${entry.slot}:${entry.blocks}`;
    if (seen.has(identity)) {
      continue;
    }
    seen.add(identity);
    out.push(entry);
  }
  out.sort((a, b) => {
    const slotDelta = (a.slot || 0) - (b.slot || 0);
    if (slotDelta !== 0) {
      return slotDelta;
    }
    return a.title.localeCompare(b.title);
  });
  return out;
}

function normalizeSystemSlug(value: string): string {
  return value.trim().toLowerCase();
}

function buildSummaryFromVersions(versions: SaveSummary[]): {
  saveCount: number;
  totalSizeBytes: number;
  latestVersion: number;
  latestCreatedAt: string;
  languageCodes: string[];
} {
  if (versions.length === 0) {
    return {
      saveCount: 0,
      totalSizeBytes: 0,
      latestVersion: 0,
      latestCreatedAt: "",
      languageCodes: []
    };
  }
  const totalSizeBytes = versions.reduce((sum, item) => sum + item.fileSize, 0);
  return {
    saveCount: versions.length,
    totalSizeBytes,
    latestVersion: versions[0].version,
    latestCreatedAt: versions[0].createdAt,
    languageCodes: mergeLanguageCodes(...versions.map((item) => item.languageCodes))
  };
}

function normalizeRegionCode(regionCode: string): string {
  const normalized = regionCode.trim().toUpperCase();
  switch (normalized) {
    case "US":
    case "USA":
      return "US";
    case "EU":
    case "EUR":
      return "EU";
    case "JP":
    case "JPN":
      return "JP";
    default:
      return "UNKNOWN";
  }
}

function regionToFlagEmoji(regionCode: string): string {
  switch (normalizeRegionCode(regionCode)) {
    case "US":
      return "🇺🇸";
    case "EU":
      return "🇪🇺";
    case "JP":
      return "🇯🇵";
    default:
      return "🌐";
  }
}

function normalizeLanguageCode(raw: string): string | null {
  const value = raw.trim().toLowerCase();
  const map: Record<string, string> = {
    en: "EN",
    eng: "EN",
    english: "EN",
    ja: "JA",
    jp: "JA",
    jpn: "JA",
    japanese: "JA",
    fr: "FR",
    fra: "FR",
    fre: "FR",
    french: "FR",
    de: "DE",
    deu: "DE",
    ger: "DE",
    german: "DE",
    es: "ES",
    spa: "ES",
    spanish: "ES",
    it: "IT",
    ita: "IT",
    italian: "IT",
    pt: "PT",
    por: "PT",
    portuguese: "PT",
    nl: "NL",
    dut: "NL",
    nld: "NL",
    dutch: "NL"
  };
  return map[value] ?? null;
}

function mergeLanguageCodes(...sources: Array<string[] | undefined>): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const source of sources) {
    if (!source) {
      continue;
    }
    for (const item of source) {
      const normalized = normalizeLanguageCode(item);
      if (!normalized || seen.has(normalized)) {
        continue;
      }
      seen.add(normalized);
      out.push(normalized);
    }
  }
  return out;
}
