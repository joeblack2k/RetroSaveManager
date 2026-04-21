import { useCallback, useMemo } from "react";
import { useParams } from "react-router-dom";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiDownloadURL } from "../../services/apiClient";
import { listSaves } from "../../services/retrosaveApi";
import { formatBytes, formatDate } from "../../utils/format";

export function SaveDetailPage(): JSX.Element {
  const params = useParams<{ saveId: string }>();
  const saveId = params.saveId ?? "";

  const loader = useCallback(() => listSaves(), []);
  const { loading, error, data } = useAsyncData(loader, []);

  const save = useMemo(() => data?.find((item) => item.id === saveId) ?? null, [data, saveId]);

  return (
    <SectionCard title="Save detail" subtitle="Inspecteer metadata en download de save payload.">
      {loading ? <LoadingState label="Save detail laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {!loading && !error && !save ? <ErrorState message="Save niet gevonden" /> : null}
      {save ? (
        <div className="stack">
          <p>
            <strong>Game:</strong> {save.game.name}
          </p>
          <p>
            <strong>Filename:</strong> {save.filename}
          </p>
          <p>
            <strong>Version:</strong> {save.version}
          </p>
          <p>
            <strong>SHA256:</strong> <code>{save.sha256}</code>
          </p>
          <p>
            <strong>File size:</strong> {formatBytes(save.fileSize)}
          </p>
          <p>
            <strong>Created:</strong> {formatDate(save.createdAt)}
          </p>
          <a className="btn btn-primary" href={apiDownloadURL(`/saves/download?id=${encodeURIComponent(save.id)}`)}>
            Download save
          </a>
        </div>
      ) : null}
    </SectionCard>
  );
}
