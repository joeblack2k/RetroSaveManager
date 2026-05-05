import type { SaveDownloadProfile, SaveSummary } from "../../../services/types";
import type { SaveRow } from "../../../utils/saveRows";

export type ConsoleGroup = {
  key: string;
  name: string;
  rows: SaveRow[];
  saveCount: number;
  totalBytes: number;
};

export type SaveSelectorState = {
  row: SaveRow;
  displayTitle: string;
  versions: SaveSummary[];
};

export type DownloadModalState = {
  title: string;
  request: { saveId: string; psLogicalKey?: string; revisionId?: string };
  profiles: SaveDownloadProfile[];
};
