import { useCallback, useMemo, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { deleteQuarantineItem, getValidationStatus, rescanValidation, retryQuarantineItem } from "../../services/retrosaveApi";
import type { QuarantineRecord, ValidationCoverageRecord, ValidationStatus } from "../../services/types";
import { formatBytes, formatDate } from "../../utils/format";

const TRUST_CARDS = [
  { key: "mediaVerified", label: "Media", detail: "known save media size/shape" },
  { key: "romVerified", label: "ROM", detail: "matched ROM evidence" },
  { key: "structureVerified", label: "Structure", detail: "parser/container verified" },
  { key: "semanticVerified", label: "Semantic", detail: "gameplay facts decoded" }
];

export function ValidationPage(): JSX.Element {
  const loader = useCallback(() => getValidationStatus(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [rescanning, setRescanning] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [quarantineAction, setQuarantineAction] = useState<string | null>(null);
  const [rescanError, setRescanError] = useState<string | null>(null);
  const [rescanMessage, setRescanMessage] = useState<string | null>(null);

  async function runRescan(dryRun = false): Promise<void> {
    if (dryRun) {
      setDryRunning(true);
    } else {
      setRescanning(true);
    }
    setRescanError(null);
    setRescanMessage(null);
    try {
      const response = await rescanValidation({ dryRun, pruneUnsupported: true });
      const result = response.result ?? {};
      setRescanMessage(
        `${dryRun ? "Dry-run" : "Rescan"} complete: ${numberOrZero(result.scanned)} scanned, ${numberOrZero(result.updated)} updated, ${numberOrZero(result.rejected)} rejected, ${numberOrZero(result.duplicateVersionsRemoved)} duplicate versions removed.`
      );
      await reload();
    } catch (err: unknown) {
      setRescanError(err instanceof Error ? err.message : "Rescan failed.");
    } finally {
      setRescanning(false);
      setDryRunning(false);
    }
  }

  async function retryQuarantine(id: string): Promise<void> {
    setQuarantineAction(`retry:${id}`);
    setRescanError(null);
    setRescanMessage(null);
    try {
      const response = await retryQuarantineItem(id);
      setRescanMessage(response.message || (response.duplicate ? "Quarantined file already matches cloud state." : "Quarantined file imported."));
      await reload();
    } catch (err: unknown) {
      setRescanError(err instanceof Error ? err.message : "Retry failed.");
      await reload();
    } finally {
      setQuarantineAction(null);
    }
  }

  async function deleteQuarantine(id: string): Promise<void> {
    const confirmed = window.confirm("Delete this quarantined file? The saved cloud versions are not touched.");
    if (!confirmed) {
      return;
    }
    setQuarantineAction(`delete:${id}`);
    setRescanError(null);
    setRescanMessage(null);
    try {
      await deleteQuarantineItem(id);
      setRescanMessage("Quarantined file deleted.");
      await reload();
    } catch (err: unknown) {
      setRescanError(err instanceof Error ? err.message : "Delete failed.");
    } finally {
      setQuarantineAction(null);
    }
  }

  return (
    <SectionCard title="Validation" subtitle="Parser trust, rejected uploads, and repair tools in one quiet place.">
      {loading ? <LoadingState label="Loading validation status..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {rescanError ? <ErrorState message={rescanError} /> : null}
      {rescanMessage ? <p className="success-state">{rescanMessage}</p> : null}
      {data ? (
        <ValidationContent
          status={data}
          rescanning={rescanning}
          dryRunning={dryRunning}
          quarantineAction={quarantineAction}
          onRescan={runRescan}
          onRetryQuarantine={retryQuarantine}
          onDeleteQuarantine={deleteQuarantine}
        />
      ) : null}
    </SectionCard>
  );
}

function ValidationContent({
  status,
  rescanning,
  dryRunning,
  quarantineAction,
  onRescan,
  onRetryQuarantine,
  onDeleteQuarantine
}: {
  status: ValidationStatus;
  rescanning: boolean;
  dryRunning: boolean;
  quarantineAction: string | null;
  onRescan: (dryRun?: boolean) => Promise<void>;
  onRetryQuarantine: (id: string) => Promise<void>;
  onDeleteQuarantine: (id: string) => Promise<void>;
}): JSX.Element {
  const topSystems = useMemo(() => {
    return Object.entries(status.systems ?? {})
      .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
      .slice(0, 8);
  }, [status.systems]);

  return (
    <div className="validation-page">
      <header className="validation-toolbar">
        <div>
          <span>Generated {formatDate(status.generatedAt)}</span>
          <strong>{status.quarantineCount} quarantined</strong>
        </div>
        <div className="validation-toolbar__actions">
          <button className="btn btn-ghost" type="button" onClick={() => void onRescan(true)} disabled={dryRunning || rescanning}>
            {dryRunning ? "Checking..." : "Dry-run repair"}
          </button>
          <button className="btn btn-primary" type="button" onClick={() => void onRescan(false)} disabled={rescanning || dryRunning}>
            {rescanning ? "Rescanning..." : "Rescan and repair"}
          </button>
        </div>
      </header>

      <div className="validation-score-grid">
        {TRUST_CARDS.map((card) => (
          <article key={card.key} className="validation-score-card">
            <span>{card.label}</span>
            <strong>{status.counts?.[card.key] ?? 0}</strong>
            <small>{card.detail}</small>
          </article>
        ))}
        <article className="validation-score-card validation-score-card--warn">
          <span>Unknown</span>
          <strong>{status.counts?.unknown ?? 0}</strong>
          <small>needs better evidence or a future parser</small>
        </article>
      </div>

      <section className="validation-section">
        <div className="validation-section__header">
          <h3>Systems</h3>
          <span>{topSystems.length} shown</span>
        </div>
        <div className="validation-system-strip">
          {topSystems.map(([system, count]) => (
            <span key={system}>
              {system} <strong>{count}</strong>
            </span>
          ))}
          {topSystems.length === 0 ? <p>No saves indexed yet.</p> : null}
        </div>
      </section>

      <CoverageSection status={status} />

      <section className="validation-section">
        <div className="validation-section__header">
          <h3>Quarantine</h3>
          <span>Retry after parser updates, or delete known junk</span>
        </div>
        {status.quarantine.length === 0 ? (
          <p className="treegrid-panel__empty">No quarantined files.</p>
        ) : (
          <div className="treegrid-table-wrap">
            <table className="treegrid-table validation-table">
              <thead>
                <tr>
                  <th>File</th>
                  <th>System</th>
                  <th>Size</th>
                  <th>Validation</th>
                  <th>Source</th>
                  <th>Reason</th>
                  <th>Date</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {status.quarantine.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.displayTitle || item.filename}</strong>
                      <small>{item.sourcePath || item.filename}</small>
                    </td>
                    <td>{item.systemSlug || "unknown"}</td>
                    <td>{formatBytes(item.sizeBytes)}</td>
                    <td>{item.trustLevel || item.parserLevel || "none"}</td>
                    <td>
                      <QuarantineSource item={item} />
                    </td>
                    <td>{item.reason}</td>
                    <td>{formatDate(item.uploadedAt)}</td>
                    <td>
                      <div className="validation-row-actions">
                        <button
                          className="btn btn-ghost"
                          type="button"
                          disabled={quarantineAction === `retry:${item.id}`}
                          onClick={() => void onRetryQuarantine(item.id)}
                        >
                          {quarantineAction === `retry:${item.id}` ? "Retrying..." : "Retry"}
                        </button>
                        <button
                          className="btn btn-ghost btn-danger"
                          type="button"
                          disabled={quarantineAction === `delete:${item.id}`}
                          onClick={() => void onDeleteQuarantine(item.id)}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}

function CoverageSection({ status }: { status: ValidationStatus }): JSX.Element {
  const summary = status.coverageSummary ?? { total: 0, gameplayFacts: 0, semantic: 0, cheats: 0, missing: 0 };
  const rows = (status.coverage ?? []).slice(0, 12);
  return (
    <section className="validation-section">
      <div className="validation-section__header">
        <h3>Gameplay Coverage</h3>
        <span>Where details and cheats are already useful</span>
      </div>
      <div className="validation-coverage-summary" aria-label="Gameplay coverage summary">
        <CoverageMetric label="Saves" value={summary.total} />
        <CoverageMetric label="Gameplay facts" value={summary.gameplayFacts} />
        <CoverageMetric label="Semantic parsers" value={summary.semantic} />
        <CoverageMetric label="Cheats" value={summary.cheats} />
        <CoverageMetric label="Needs parser" value={summary.missing} warn />
      </div>
      {rows.length === 0 ? (
        <p className="treegrid-panel__empty">No coverage data yet.</p>
      ) : (
        <div className="validation-coverage-list">
          {rows.map((item) => (
            <CoverageRow key={item.saveId} item={item} />
          ))}
        </div>
      )}
    </section>
  );
}

function CoverageMetric({ label, value, warn }: { label: string; value: number; warn?: boolean }): JSX.Element {
  return (
    <div className={warn ? "validation-coverage-metric validation-coverage-metric--warn" : "validation-coverage-metric"}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function CoverageRow({ item }: { item: ValidationCoverageRecord }): JSX.Element {
  const state = item.hasGameplayFacts ? `${item.gameplayFactCount} facts` : item.cheatsSupported ? `${item.cheatCount ?? 0} cheats` : "missing parser";
  return (
    <article className="validation-coverage-row">
      <div>
        <strong>{item.displayTitle}</strong>
        <small>
          {item.systemName || item.systemSlug} · {item.parserId || item.parserLevel || "no parser"}
        </small>
      </div>
      <span className={item.hasGameplayFacts || item.cheatsSupported ? "validation-pill validation-pill--good" : "validation-pill"}>{state}</span>
    </article>
  );
}

function QuarantineSource({ item }: { item: QuarantineRecord }): JSX.Element {
  const parts = [item.uploadSource, item.runtimeProfile, item.format || item.mediaType].filter(Boolean);
  if (parts.length === 0) {
    return <span>-</span>;
  }
  return (
    <span className="validation-source-cell" title={parts.join(" / ")}>
      {parts.join(" / ")}
    </span>
  );
}

function numberOrZero(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
