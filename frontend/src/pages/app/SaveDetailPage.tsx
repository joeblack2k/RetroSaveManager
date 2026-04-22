import { useCallback, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { getSaveHistory, rollbackSave } from "../../services/retrosaveApi";
import type { MemoryCardEntry, SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";
import { buildSaveDetailsHref } from "../../utils/saveRows";

export function SaveDetailPage(): JSX.Element {
  const params = useParams<{ saveId: string }>();
  const [searchParams] = useSearchParams();
  const saveId = params.saveId ?? "";
  const psLogicalKey = (searchParams.get("psLogicalKey") || "").trim();
  const [rollbackError, setRollbackError] = useState<string | null>(null);
  const [rollbackMessage, setRollbackMessage] = useState<string | null>(null);
  const [rollbackingId, setRollbackingId] = useState<string | null>(null);

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
    const confirmed = window.confirm(`Rollback naar versie ${target.version} van ${displayTitle}?\n\nEr wordt een nieuwe nieuwste versie aangemaakt (promote copy).`);
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
      setRollbackMessage(`Rollback voltooid. Nieuwe versie: v${response.save.version}`);
      await reload();
    } catch (err: unknown) {
      setRollbackError(err instanceof Error ? err.message : "Rollback mislukt");
    } finally {
      setRollbackingId(null);
    }
  }

  return (
    <SectionCard title="Save details & history" subtitle="Bekijk metadata, versiehistorie en rollback-opties voor deze savegame.">
      {loading ? <LoadingState label="Save history laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {!loading && !error && versions.length === 0 ? <ErrorState message="Geen versies gevonden voor deze save." /> : null}

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
                            <a
                              className="saves-action-link"
                              href={apiDownloadURL(`/saves/download?id=${encodeURIComponent(saveId)}&psLogicalKey=${encodeURIComponent(entryLogicalKey)}`)}
                            >
                              Download
                            </a>
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
                        <a
                          className="saves-action-link"
                          href={apiDownloadURL(
                            effectiveLogicalKey
                              ? `/saves/download?id=${encodeURIComponent(saveId)}&psLogicalKey=${encodeURIComponent(effectiveLogicalKey)}&revisionId=${encodeURIComponent(version.id)}`
                              : `/saves/download?id=${encodeURIComponent(version.id)}`
                          )}
                        >
                          Download
                        </a>
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
