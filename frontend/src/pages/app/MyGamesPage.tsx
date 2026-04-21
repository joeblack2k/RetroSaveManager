import { useCallback } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { listLibrary, listSaves } from "../../services/retrosaveApi";
import { formatDate } from "../../utils/format";

export function MyGamesPage(): JSX.Element {
  const loader = useCallback(async () => {
    const [library, saves] = await Promise.all([listLibrary(), listSaves()]);
    return { library, saves };
  }, []);

  const { loading, error, data } = useAsyncData(loader, []);

  return (
    <div className="grid two-cols">
      <SectionCard title="My games" subtitle="Games in je persoonlijke library.">
        {loading ? <LoadingState label="Library laden..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list">
            {data.library.map((entry) => (
              <li key={entry.id}>
                <strong>{entry.catalog.name}</strong>
                <p>{entry.catalog.system.name}</p>
                <small>Added: {formatDate(entry.addedAt)}</small>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>
      <SectionCard title="Recente saves" subtitle="Snel naar save details.">
        {loading ? <LoadingState label="Saves laden..." /> : null}
        {error ? <ErrorState message={error} /> : null}
        {data ? (
          <ul className="plain-list">
            {data.saves.slice(0, 10).map((save) => (
              <li key={save.id}>
                <strong>{save.game.name}</strong>
                <p>{save.filename}</p>
                <small>{formatDate(save.createdAt)}</small>
                <div>
                  <Link to={`/app/saves/${save.id}`} className="text-link">
                    Open save detail
                  </Link>
                </div>
              </li>
            ))}
          </ul>
        ) : null}
      </SectionCard>
    </div>
  );
}
