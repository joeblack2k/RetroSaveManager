export type ApiEnvelope<T> = T & { success?: boolean; message?: string };

export type UserQuota = {
  storage: { status: string; used?: number; limit?: number };
  devices: { status: string; used?: number; limit?: number };
};

export type AuthUser = {
  id: string;
  email: string;
  gameCount: number;
  fileCount: number;
  storageUsedBytes: number;
  quota?: UserQuota;
};

export type SaveSystem = {
  id: number;
  name: string;
  slug?: string;
};

export type SaveGame = {
  id: number;
  name: string;
  boxart: string | null;
  boxartThumb: string | null;
  hasParser: boolean;
  system: SaveSystem | null;
};

export type SaveSummary = {
  id: string;
  game: SaveGame;
  filename: string;
  fileSize: number;
  format: string;
  version: number;
  sha256: string;
  createdAt: string;
  metadata: unknown;
};

export type Device = {
  id: number;
  deviceType: string;
  fingerprint: string;
  alias: string | null;
  displayName: string;
  lastSyncedAt: string;
  createdAt: string;
};

export type Conflict = {
  id: string;
  game: {
    name: string;
    boxartThumb: string | null;
  };
  deviceName: string | null;
  deviceFilename: string;
  deviceFileSize: number;
  createdAt: string;
  cloudLatest: {
    filename: string;
    fileSize: number;
    version: number;
    createdAt: string;
    metadata?: { summary?: string };
  } | null;
};

export type CatalogItem = {
  id: string;
  name: string;
  description: string;
  system: SaveSystem;
  downloadUrl: string;
};

export type LibraryEntry = {
  id: string;
  catalog: CatalogItem;
  addedAt: string;
};

export type RoadmapItem = {
  id: string;
  title: string;
  description: string;
  votes: number;
  createdAt: string;
};

export type RoadmapSuggestion = {
  id: string;
  title: string;
  description: string;
  status: string;
  createdAt: string;
};

export type ReferralInfo = {
  code: string;
  url: string;
  stats: {
    referrals: number;
    credits: number;
  };
};
