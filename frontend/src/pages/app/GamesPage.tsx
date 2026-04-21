import { useCallback } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { listSaves } from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";

export function GamesPage(): JSX.Element {
  const loader = useCallback(() => listSaves(), []);
  const { loading, error, data } = useAsyncData(loader, []);

  return (
    <SectionCard title="Games & saves" subtitle="Volledige save-lijst uit de compat API.">
      {loading ? <LoadingState label="Saves laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <table className="table">
          <thead>
            <tr>
              <th>Game</th>
              <th>Region</th>
              <th>Bestand</th>
              <th>Versie</th>
              <th>Grootte</th>
              <th>Aangemaakt</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {data.map((save) => (
              <tr key={save.id}>
                <td>{save.displayTitle || save.game.displayTitle || save.game.name}</td>
                <td>{regionToFlagEmoji((save.regionCode || save.game.regionCode || "UNKNOWN").toString())}</td>
                <td>{save.filename}</td>
                <td>{save.version}</td>
                <td>{formatBytes(save.fileSize)}</td>
                <td>{formatDate(save.createdAt)}</td>
                <td>
                  <Link className="text-link" to={`/app/saves/${save.id}`}>
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

function regionToFlagEmoji(regionCode: string): string {
  switch (regionCode.toUpperCase()) {
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
