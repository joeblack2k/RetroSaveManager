import { useCallback } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { apiBaseForUi } from "../../services/apiClient";
import { getCurrentUser } from "../../services/retrosaveApi";
import { formatBytes } from "../../utils/format";

export function SettingsPage(): JSX.Element {
  const loader = useCallback(() => getCurrentUser(), []);
  const { loading, error, data } = useAsyncData(loader, []);

  return (
    <SectionCard title="Settings" subtitle="Runtime en compatibiliteitsinstellingen voor deze web app.">
      <p>
        <strong>API base:</strong> <code>{apiBaseForUi()}</code>
      </p>
      <p>
        <strong>Auth mode:</strong> <code>disabled</code>
      </p>
      {loading ? <LoadingState label="Gebruiker laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <div className="stack compact">
          <p>
            <strong>User:</strong> {data.email}
          </p>
          <p>
            <strong>Storage used:</strong> {formatBytes(data.storageUsedBytes)}
          </p>
          <p>
            <strong>Games:</strong> {data.gameCount}
          </p>
          <p>
            <strong>Files:</strong> {data.fileCount}
          </p>
        </div>
      ) : null}
    </SectionCard>
  );
}
