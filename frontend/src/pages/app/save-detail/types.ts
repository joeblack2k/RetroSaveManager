import type { SaveDownloadProfile } from "../../../services/types";

export type DownloadModalState = {
  title: string;
  request: { saveId: string; psLogicalKey?: string; revisionId?: string };
  profiles: SaveDownloadProfile[];
};

export type DetailMetric = {
  label: string;
  value: string;
};

export type OpenDownloadModal = (
  title: string,
  request: { saveId: string; psLogicalKey?: string; revisionId?: string },
  profiles: SaveDownloadProfile[] | undefined
) => void;
