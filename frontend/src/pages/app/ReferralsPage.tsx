import { useCallback } from "react";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { getReferral } from "../../services/retrosaveApi";

export function ReferralsPage(): JSX.Element {
  const loader = useCallback(() => getReferral(), []);
  const { loading, error, data } = useAsyncData(loader, []);

  return (
    <SectionCard title="Referrals" subtitle="Compatibele referral endpoint output.">
      {loading ? <LoadingState label="Referral data laden..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {data ? (
        <div className="stack compact">
          <p>
            <strong>Code:</strong> <code>{data.code}</code>
          </p>
          <p>
            <strong>URL:</strong> <code>{data.url}</code>
          </p>
          <p>
            <strong>Referrals:</strong> {data.stats.referrals}
          </p>
          <p>
            <strong>Credits:</strong> {data.stats.credits}
          </p>
        </div>
      ) : null}
    </SectionCard>
  );
}
