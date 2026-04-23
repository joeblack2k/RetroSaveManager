import { useCallback, useState } from "react";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { SectionCard } from "../../components/SectionCard";
import { useAsyncData } from "../../hooks/useAsyncData";
import { listSyncLogs } from "../../services/retrosaveApi";
import { formatDate } from "../../utils/format";

const LOG_PAGE_SIZE = 50;
const LOG_LOOKBACK_HOURS = 72;

export function LogsPage(): JSX.Element {
  const [page, setPage] = useState(1);
  const loader = useCallback(() => listSyncLogs({ hours: LOG_LOOKBACK_HOURS, page, limit: LOG_PAGE_SIZE }), [page]);
  const { loading, error, data } = useAsyncData(loader, [page]);

  return (
    <SectionCard title="Logs" subtitle="Last 72 hours of sync activity across helper uploads, downloads, rollbacks, deletes, rescans, and conflicts.">
      {loading ? <LoadingState label="Loading logs..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <div className="logs-panel">
          <div className="logs-panel__meta">
            <span>{data.total} rows</span>
            <span>{data.totalPages} pages</span>
          </div>

          {data.logs.length === 0 ? <p className="logs-panel__empty">No sync activity was recorded in the last 72 hours.</p> : null}

          {data.logs.length > 0 ? (
            <div className="logs-table-wrap">
              <table className="logs-table">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Device</th>
                    <th>Action</th>
                    <th>Game</th>
                    <th>Error</th>
                  </tr>
                </thead>
                <tbody>
                  {data.logs.map((entry) => (
                    <tr key={entry.id}>
                      <td>{formatDate(entry.createdAt)}</td>
                      <td>{entry.deviceName}</td>
                      <td>{formatLogAction(entry.action)}</td>
                      <td>
                        <div className="logs-table__game">{entry.game}</div>
                        {entry.errorMessage ? <div className="logs-table__message">{entry.errorMessage}</div> : null}
                      </td>
                      <td>
                        <span className={entry.error ? "logs-badge logs-badge--error" : "logs-badge logs-badge--ok"}>
                          {entry.error ? "Yes" : "No"}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}

          <div className="logs-pagination">
            <button className="btn btn-ghost" type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={page <= 1}>
              Previous
            </button>
            <span>
              Page {data.page} of {data.totalPages}
            </span>
            <button
              className="btn btn-ghost"
              type="button"
              onClick={() => setPage((current) => Math.min(data.totalPages, current + 1))}
              disabled={page >= data.totalPages}
            >
              Next
            </button>
          </div>
        </div>
      ) : null}
    </SectionCard>
  );
}

function formatLogAction(action: string): string {
  switch (action) {
    case "download_many":
      return "Download many";
    case "conflict_resolved":
      return "Conflict resolved";
    default:
      return action
        .split("_")
        .map((part) => (part ? `${part[0].toUpperCase()}${part.slice(1)}` : part))
        .join(" ");
  }
}
