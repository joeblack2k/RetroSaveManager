import { useCallback } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { listConflicts } from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";

export function ConflictsPage(): JSX.Element {
  const loader = useCallback(() => listConflicts(), []);
  const { loading, error, data } = useAsyncData(loader, []);

  return (
    <SectionCard title="Conflicts" subtitle="Overzicht van actieve conflictmeldingen uit `/conflicts`.">
      {loading ? <LoadingState label="Conflicts laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <ul className="plain-list">
          {data.map((conflict) => (
            <li key={conflict.id}>
              <strong>{conflict.game.name}</strong>
              <p>{conflict.deviceFilename}</p>
              <p>Device: {conflict.deviceName ?? "unknown"}</p>
              <p>Size: {formatBytes(conflict.deviceFileSize)}</p>
              <small>{formatDate(conflict.createdAt)}</small>
            </li>
          ))}
        </ul>
      ) : null}
    </SectionCard>
  );
}
