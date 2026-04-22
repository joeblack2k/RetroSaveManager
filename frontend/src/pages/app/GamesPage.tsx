import { useCallback, useMemo } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { listSaves } from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";
import { buildSaveDetailsHref, buildSaveRows, type SaveRow } from "../../utils/saveRows";

export function GamesPage(): JSX.Element {
  const loader = useCallback(() => listSaves(), []);
  const { loading, error, data } = useAsyncData(loader, []);
  const rows = useMemo<SaveRow[]>(() => buildSaveRows(data ?? []), [data]);

  return (
    <SectionCard title="Games & saves" subtitle="Canonieke save-lijst per game uit de compat API.">
      {loading ? <LoadingState label="Saves laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <table className="table">
          <thead>
            <tr>
              <th>Game</th>
              <th>Console</th>
              <th>Region</th>
              <th>Saves</th>
              <th>Latest</th>
              <th>Total</th>
              <th>Aangemaakt</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key}>
                <td>{row.gameName}</td>
                <td>{row.systemName}</td>
                <td>{row.regionFlag}</td>
                <td>{row.saveCount}</td>
                <td>{formatBytes(row.latestSizeBytes)}</td>
                <td>{formatBytes(row.totalBytes)}</td>
                <td>{formatDate(row.latestCreatedAt)}</td>
                <td>
                  <Link className="text-link" to={buildSaveDetailsHref(row)}>
                    Details
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
    </SectionCard>
  );
}
