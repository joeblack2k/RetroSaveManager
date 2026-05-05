import { useCallback, useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getSaveCheats, getSaveHistory, rollbackSave } from "../../services/retrosaveApi";
import type { SaveCheatEditorState, SaveDownloadProfile, SaveSummary } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";
import { buildSaveInsight } from "../../utils/saveInsights";
import { DecodedSavePanel } from "./save-detail/DecodedSavePanel";
import { DownloadSaveDialog } from "./save-detail/DownloadSaveDialog";
import { LogicalSavePanel } from "./save-detail/LogicalSavePanel";
import { PlayStationSavePicker } from "./save-detail/PlayStationSavePicker";
import { SaveDetailHero } from "./save-detail/SaveDetailHero";
import { TechnicalDetailsPanel } from "./save-detail/TechnicalDetailsPanel";
import { VersionHistoryTable } from "./save-detail/VersionHistoryTable";
import {
  buildSummaryFromVersions,
  mergeLanguageCodes,
  normalizeRegionCode,
  normalizeSystemSlug,
  regionToFlagEmoji,
  uniqueMemoryCardEntries
} from "./save-detail/helpers";
import type { DetailMetric, DownloadModalState } from "./save-detail/types";

export function SaveDetailPage(): JSX.Element {
  const params = useParams<{ saveId: string }>();
  const [searchParams] = useSearchParams();
  const saveId = params.saveId ?? "";
  const psLogicalKey = (searchParams.get("psLogicalKey") || "").trim();
  const [rollbackError, setRollbackError] = useState<string | null>(null);
  const [rollbackMessage, setRollbackMessage] = useState<string | null>(null);
  const [rollbackingId, setRollbackingId] = useState<string | null>(null);
  const [pendingRollback, setPendingRollback] = useState<SaveSummary | null>(null);
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
  const cheatRequest =
    latest && hasCheats && !showPlayStationSelector
      ? {
          saveId: effectiveLogicalKey ? saveId : latest.id,
          psLogicalKey: effectiveLogicalKey || undefined
        }
      : null;
  const cheatLoader = useCallback(async () => {
    if (!cheatRequest) {
      return null;
    }
    const response = await getSaveCheats(cheatRequest.saveId, cheatRequest.psLogicalKey);
    return response.cheats?.supported ? response.cheats : null;
  }, [cheatRequest?.saveId, cheatRequest?.psLogicalKey]);
  const { loading: cheatLoading, error: cheatError, data: cheatData } = useAsyncData<SaveCheatEditorState | null>(cheatLoader, [cheatRequest?.saveId, cheatRequest?.psLogicalKey]);
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
      setPendingRollback(null);
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
          <SaveDetailHero
            displayTitle={displayTitle}
            systemName={systemName}
            showPlayStationSelector={showPlayStationSelector}
            latestVersion={latestVersion}
            latestCreatedAt={latestCreatedAt}
            heroMetrics={heroMetrics}
            hasCheats={hasCheats}
            parserLevel={saveInsight?.parserLevel}
            currentDownloadRequest={currentDownloadRequest}
            downloadProfiles={latest?.downloadProfiles}
            openDownloadModal={openDownloadModal}
          />

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
                cheatLoading={Boolean(cheatRequest) && cheatLoading}
                cheatError={Boolean(cheatRequest) ? cheatError : null}
              />
              {logicalEntry ? <LogicalSavePanel entry={logicalEntry} latest={latest} /> : null}
              <TechnicalDetailsPanel latest={latest} insight={saveInsight} languageCodes={languageCodes} />
              <VersionHistoryTable
                versions={versions}
                displayTitle={displayTitle}
                effectiveLogicalKey={effectiveLogicalKey}
                saveId={saveId}
                rollbackingId={rollbackingId}
                requestRollback={setPendingRollback}
                openDownloadModal={openDownloadModal}
              />
            </>
          )}
        </div>
      ) : null}

      {downloadState ? <DownloadSaveDialog state={downloadState} onClose={closeDownloadModal} /> : null}

      {pendingRollback ? (
        <ConfirmDialog
          title={`Rollback ${displayTitle}?`}
          message={`Version ${pendingRollback.version} will be promoted by creating a new latest sync copy. Existing history stays available.`}
          confirmLabel="Rollback"
          busy={rollbackingId === pendingRollback.id}
          onConfirm={() => void handleRollback(pendingRollback)}
          onCancel={() => setPendingRollback(null)}
        />
      ) : null}
    </SectionCard>
  );
}
