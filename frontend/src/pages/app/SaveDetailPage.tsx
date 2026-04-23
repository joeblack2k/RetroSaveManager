import { useCallback, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getSaveHistory, rollbackSave } from "../../services/retrosaveApi";
import type { MemoryCardEntry, SaveDownloadProfile, SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";
import { buildSaveDetailsHref, buildSaveDownloadHref } from "../../utils/saveRows";

type DownloadModalState = {
  title: string;
  request: { saveId: string; psLogicalKey?: string; revisionId?: string };
  profiles: SaveDownloadProfile[];
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
  const systemName = data?.summary?.system?.name || latest?.game.system?.name || "Unknown";
  const regionCode = normalizeRegionCode((data?.summary?.regionCode || latest?.regionCode || latest?.game.regionCode || logicalEntry?.regionCode || "UNKNOWN").toString());
  const languageCodes = mergeLanguageCodes(data?.summary?.languageCodes, latest?.languageCodes, latest?.game.languageCodes, fallbackSummary.languageCodes);
  const saveCount = showPlayStationSelector ? playStationEntries.length : data?.summary?.saveCount || fallbackSummary.saveCount;
  const totalSizeBytes = showPlayStationSelector
    ? playStationEntries.reduce((sum, entry) => sum + (entry.totalSizeBytes || entry.sizeBytes || 0), 0)
    : data?.summary?.totalSizeBytes || fallbackSummary.totalSizeBytes;
  const latestVersion = showPlayStationSelector ? 0 : data?.summary?.latestVersion || fallbackSummary.latestVersion;
  const latestCreatedAt = data?.summary?.latestCreatedAt || fallbackSummary.latestCreatedAt;

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
      setRollbackMessage(`Rollback complete. New version: v${response.save.version}`);
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
    <SectionCard title="Save details & history" subtitle="Inspect metadata, version history, downloads, and rollback options for this save.">
      {loading ? <LoadingState label="Loading save history..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {!loading && !error && versions.length === 0 ? <ErrorState message="No versions found for this save." /> : null}

      {rollbackError ? <p className="error-state">{rollbackError}</p> : null}
      {rollbackMessage ? <p className="success-state">{rollbackMessage}</p> : null}

      {!loading && !error && versions.length > 0 ? (
        <div className="stack">
          <div className="stack compact">
            <p><strong>Game:</strong> {displayTitle}</p>
            <p><strong>Console:</strong> {systemName}</p>
            {!showPlayStationSelector ? <p><strong>Region:</strong> {regionToFlagEmoji(regionCode)} {regionCode}</p> : null}
            {!showPlayStationSelector ? <p><strong>Languages:</strong> {languageCodes.length > 0 ? languageCodes.join(", ") : "-"}</p> : null}
            <p><strong>{showPlayStationSelector ? "Available saves" : "Total saves"}:</strong> {saveCount}</p>
            <p><strong>Total size:</strong> {formatBytes(totalSizeBytes)}</p>
            {!showPlayStationSelector ? <p><strong>Latest version:</strong> v{latestVersion}</p> : null}
            <p><strong>Latest date:</strong> {formatDate(latestCreatedAt)}</p>
          </div>

          {showPlayStationSelector ? (
            <div className="stack compact">
              <p>This PlayStation upload contains individual game saves. Open the game you want to inspect instead of the projection container.</p>
              <table className="table">
                <thead>
                  <tr>
                    <th>Preview</th>
                    <th>Save</th>
                    <th>Region</th>
                    <th>Size</th>
                    <th>Details</th>
                    <th>Download</th>
                  </tr>
                </thead>
                <tbody>
                  {playStationEntries.map((entry, index) => {
                    const entryLogicalKey = (entry.logicalKey || "").trim();
                    const canOpen = entryLogicalKey !== "";
                    return (
                      <tr key={`${entryLogicalKey || entry.productCode || entry.directoryName || entry.title}-${index}`}>
                        <td>
                          {entry.iconDataUrl ? (
                            <img
                              className="memory-card-entry-preview"
                              src={entry.iconDataUrl}
                              alt={`${entry.title} icon`}
                              loading="lazy"
                            />
                          ) : (
                            <div className="memory-card-entry-preview memory-card-entry-preview--empty" aria-hidden="true" />
                          )}
                        </td>
                        <td>
                          <div className="memory-card-entry-title-cell">
                            <strong>{entry.title}</strong>
                            {entry.productCode ? <span>{entry.productCode}</span> : entry.directoryName ? <span>{entry.directoryName}</span> : null}
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
                              onClick={() =>
                                openDownloadModal(entry.title, { saveId, psLogicalKey: entryLogicalKey }, latest?.downloadProfiles)
                              }
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
          ) : logicalEntry ? (
            <div className="stack compact">
              {logicalEntry.iconDataUrl ? (
                <img
                  className="memory-card-entry-preview"
                  src={logicalEntry.iconDataUrl}
                  alt={`${logicalEntry.title} icon`}
                  loading="lazy"
                />
              ) : null}
              {logicalEntry.productCode ? <p><strong>Product code:</strong> {logicalEntry.productCode}</p> : null}
              {logicalEntry.directoryName ? <p><strong>Directory:</strong> {logicalEntry.directoryName}</p> : null}
              {logicalEntry.slot > 0 ? <p><strong>Slot:</strong> {logicalEntry.slot}</p> : null}
              {logicalEntry.blocks > 0 ? <p><strong>Blocks:</strong> {logicalEntry.blocks}</p> : null}
              <p><strong>Current save size:</strong> {logicalEntry.sizeBytes ? formatBytes(logicalEntry.sizeBytes) : formatBytes(latest?.fileSize || 0)}</p>
            </div>
          ) : null}

          {!showPlayStationSelector ? (
            <table className="table">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Date</th>
                  <th>File</th>
                  <th>Size</th>
                  <th>SHA256</th>
                  <th>Region</th>
                  <th>Languages</th>
                  <th>Download</th>
                  <th>Rollback</th>
                </tr>
              </thead>
              <tbody>
                {versions.map((version, index) => {
                  const isLatest = index === 0;
                  const versionRegion = normalizeRegionCode((version.regionCode || version.game.regionCode || "UNKNOWN").toString());
                  const versionLanguages = mergeLanguageCodes(version.languageCodes, version.game.languageCodes);
                  const isBusy = rollbackingId === version.id;
                  return (
                    <tr key={version.id}>
                      <td>v{version.version}</td>
                      <td>{formatDate(version.createdAt)}</td>
                      <td>{version.filename}</td>
                      <td>{formatBytes(version.fileSize)}</td>
                      <td><code>{version.sha256}</code></td>
                      <td>{regionToFlagEmoji(versionRegion)} {versionRegion}</td>
                      <td>{versionLanguages.length > 0 ? versionLanguages.join(", ") : "-"}</td>
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
                        <button
                          className="saves-action-btn"
                          type="button"
                          disabled={isLatest || isBusy}
                          onClick={() => void handleRollback(version)}
                        >
                          {isLatest ? "Current" : isBusy ? "..." : "Rollback"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : null}
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
