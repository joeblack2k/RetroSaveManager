import { Link } from "react-router-dom";
import type { SaveDownloadProfile } from "../../../services/types";
import { formatDate } from "../../../utils/format";
import type { DetailMetric, OpenDownloadModal } from "./types";

type SaveDetailHeroProps = {
  displayTitle: string;
  systemName: string;
  showPlayStationSelector: boolean;
  latestVersion: number;
  latestCreatedAt: string;
  heroMetrics: DetailMetric[];
  hasCheats: boolean;
  parserLevel: string | undefined;
  currentDownloadRequest: { saveId: string; psLogicalKey?: string; revisionId?: string } | null;
  downloadProfiles: SaveDownloadProfile[] | undefined;
  openDownloadModal: OpenDownloadModal;
};

export function SaveDetailHero({
  displayTitle,
  systemName,
  showPlayStationSelector,
  latestVersion,
  latestCreatedAt,
  heroMetrics,
  hasCheats,
  parserLevel,
  currentDownloadRequest,
  downloadProfiles,
  openDownloadModal
}: SaveDetailHeroProps): JSX.Element {
  return (
    <header className="save-detail-hero">
      <div className="save-detail-hero__main">
        <Link className="save-detail-back" to="/app/my-games">&lt; My Saves</Link>
        <p className="save-detail-eyebrow">{systemName} / current sync save</p>
        <h2>{displayTitle}</h2>
        <p className="save-detail-subtitle">
          {showPlayStationSelector
            ? "This memory-card upload contains multiple game saves. Select one to inspect its own history."
            : `Version ${latestVersion} is leading for sync. Last updated ${formatDate(latestCreatedAt)}.`}
        </p>
        <div className="save-detail-metrics" aria-label="Save summary">
          {heroMetrics.map((metric) => (
            <div className="save-detail-metric" key={metric.label}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </div>
          ))}
        </div>
      </div>
      <div className="save-detail-actions">
        {hasCheats ? <span className="save-detail-tag save-detail-tag--cheats">Cheats available</span> : null}
        {parserLevel ? <span className="save-detail-tag">{parserLevel} verified</span> : null}
        {currentDownloadRequest ? (
          <button className="save-detail-primary-btn" type="button" onClick={() => openDownloadModal(displayTitle, currentDownloadRequest, downloadProfiles)}>
            Download current
          </button>
        ) : null}
      </div>
    </header>
  );
}
