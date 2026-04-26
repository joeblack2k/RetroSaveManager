import { useCallback, useMemo, useState } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getValidationStatus, rescanValidation } from "../../services/retrosaveApi";
import type { ValidationStatus } from "../../services/types";
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
  const [rescanError, setRescanError] = useState<string | null>(null);
  const [rescanMessage, setRescanMessage] = useState<string | null>(null);

  async function runRescan(): Promise<void> {
    setRescanning(true);
    setRescanError(null);
    setRescanMessage(null);
    try {
      const response = await rescanValidation({ dryRun: false, pruneUnsupported: true });
      const result = response.result ?? {};
      setRescanMessage(
        `Rescan complete: ${numberOrZero(result.scanned)} scanned, ${numberOrZero(result.updated)} updated, ${numberOrZero(result.rejected)} rejected, ${numberOrZero(result.duplicateVersionsRemoved)} duplicate versions removed.`
      );
      await reload();
    } catch (err: unknown) {
      setRescanError(err instanceof Error ? err.message : "Rescan failed.");
    } finally {
      setRescanning(false);
    }
  }

  return (
    <SectionCard title="Validation" subtitle="Parser trust, rejected uploads, and repair tools in one quiet place.">
      {loading ? <LoadingState label="Loading validation status..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {rescanError ? <ErrorState message={rescanError} /> : null}
      {rescanMessage ? <p className="success-state">{rescanMessage}</p> : null}
      {data ? <ValidationContent status={data} rescanning={rescanning} onRescan={runRescan} /> : null}
    </SectionCard>
  );
}

function ValidationContent({ status, rescanning, onRescan }: { status: ValidationStatus; rescanning: boolean; onRescan: () => Promise<void> }): JSX.Element {
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
        <button className="btn btn-primary" type="button" onClick={() => void onRescan()} disabled={rescanning}>
          {rescanning ? "Rescanning..." : "Rescan and repair"}
        </button>
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

      <section className="validation-section">
        <div className="validation-section__header">
          <h3>Quarantine</h3>
          <span>Rejected uploads stay out of My Saves</span>
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
                  <th>Reason</th>
                  <th>Date</th>
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
                    <td>{item.reason}</td>
                    <td>{formatDate(item.uploadedAt)}</td>
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

function numberOrZero(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
