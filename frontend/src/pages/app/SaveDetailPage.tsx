import { useCallback, useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { getSaveHistory, rollbackSave } from "../../services/retrosaveApi";
import type { SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";

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

  const displayTitle = (data?.displayTitle || data?.summary?.displayTitle || latest?.displayTitle || latest?.game.displayTitle || latest?.game.name || "Unknown game").trim();
  const systemName = data?.summary?.system?.name || latest?.game.system?.name || "Unknown";
  const regionCode = normalizeRegionCode((data?.summary?.regionCode || latest?.regionCode || latest?.game.regionCode || "UNKNOWN").toString());
  const languageCodes = mergeLanguageCodes(data?.summary?.languageCodes, latest?.languageCodes, latest?.game.languageCodes, fallbackSummary.languageCodes);
  const saveCount = data?.summary?.saveCount || fallbackSummary.saveCount;
  const totalSizeBytes = data?.summary?.totalSizeBytes || fallbackSummary.totalSizeBytes;
  const latestVersion = data?.summary?.latestVersion || fallbackSummary.latestVersion;
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
        psLogicalKey
          ? { saveId, psLogicalKey, revisionId: target.id }
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
            <p><strong>Region:</strong> {regionToFlagEmoji(regionCode)} {regionCode}</p>
            <p><strong>Languages:</strong> {languageCodes.length > 0 ? languageCodes.join(", ") : "-"}</p>
            <p><strong>Total saves:</strong> {saveCount}</p>
            <p><strong>Total size:</strong> {formatBytes(totalSizeBytes)}</p>
            <p><strong>Latest version:</strong> v{latestVersion}</p>
            <p><strong>Latest date:</strong> {formatDate(latestCreatedAt)}</p>
          </div>

          {memoryCard ? (
            <div className="stack compact">
              <p><strong>{psLogicalKey ? "Source card" : "Memory Card"}:</strong> {memoryCard.name}</p>
              {memoryCard.entries && memoryCard.entries.length > 0 ? (
                <table className="table">
                  <thead>
                    <tr>
                      <th>Preview</th>
                      <th>Save</th>
                      <th>Slot</th>
                      <th>Blocks</th>
                      <th>Size</th>
                      <th>Code</th>
                      <th>Region</th>
                    </tr>
                  </thead>
                  <tbody>
                    {memoryCard.entries.map((entry, index) => (
                      <tr key={`${entry.productCode || entry.title}-${entry.slot}-${index}`}>
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
                            {entry.directoryName ? <span>{entry.directoryName}</span> : null}
                          </div>
                        </td>
                        <td>{entry.slot}</td>
                        <td>{entry.blocks}</td>
                        <td>{entry.sizeBytes ? formatBytes(entry.sizeBytes) : "-"}</td>
                        <td>{entry.productCode || "-"}</td>
                        <td>{regionToFlagEmoji((entry.regionCode || "UNKNOWN").toString())} {normalizeRegionCode((entry.regionCode || "UNKNOWN").toString())}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p>Geen memory card entries gevonden.</p>
              )}
            </div>
          ) : null}

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
                          psLogicalKey
                            ? `/saves/download?id=${encodeURIComponent(saveId)}&psLogicalKey=${encodeURIComponent(psLogicalKey)}&revisionId=${encodeURIComponent(version.id)}`
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
        </div>
      ) : null}
    </SectionCard>
  );
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
  const seen = new Set<string>();
  const out: string[] = [];
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
