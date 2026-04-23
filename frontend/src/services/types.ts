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
  manufacturer?: string;
};

export type SaveGame = {
  id: number;
  name: string;
  displayTitle?: string;
  regionCode?: "US" | "EU" | "JP" | "UNKNOWN" | string;
  regionFlag?: string;
  languageCodes?: string[];
  coverArtUrl?: string;
  boxart: string | null;
  boxartThumb: string | null;
  hasParser: boolean;
  system: SaveSystem | null;
};

export type MemoryCardEntry = {
  logicalKey?: string;
  title: string;
  slot: number;
  blocks: number;
  productCode?: string;
  regionCode?: "US" | "EU" | "JP" | "UNKNOWN" | string;
  directoryName?: string;
  iconDataUrl?: string;
  sizeBytes?: number;
  saveCount?: number;
  latestVersion?: number;
  latestSizeBytes?: number;
  totalSizeBytes?: number;
  latestCreatedAt?: string;
  portable?: boolean;
};

export type MemoryCardDetails = {
  name: string;
  entries?: MemoryCardEntry[];
};

export type SaveSummary = {
  id: string;
  game: SaveGame;
  displayTitle?: string;
  logicalKey?: string;
  systemSlug?: string;
  regionCode?: "US" | "EU" | "JP" | "UNKNOWN" | string;
  regionFlag?: string;
  languageCodes?: string[];
  coverArtUrl?: string;
  saveCount?: number;
  latestSizeBytes?: number;
  totalSizeBytes?: number;
  latestVersion?: number;
  memoryCard?: MemoryCardDetails | null;
  filename: string;
  fileSize: number;
  format: string;
  version: number;
  sha256: string;
  createdAt: string;
  metadata: unknown;
};

export type SaveHistorySummary = {
  displayTitle: string;
  system: SaveSystem | null;
  regionCode: "US" | "EU" | "JP" | "UNKNOWN" | string;
  regionFlag: string;
  languageCodes?: string[];
  saveCount: number;
  totalSizeBytes: number;
  latestVersion: number;
  latestCreatedAt: string;
};

export type SaveHistoryResponse = {
  success: boolean;
  game: SaveGame | null;
  displayTitle?: string;
  systemSlug?: string;
  summary?: SaveHistorySummary;
  versions: SaveSummary[];
};

export type Device = {
  id: number;
  deviceType: string;
  fingerprint: string;
  alias: string | null;
  displayName: string;
  hostname?: string;
  helperName?: string;
  helperVersion?: string;
  platform?: string;
  syncPaths?: string[];
  reportedSystemSlugs?: string[];
  lastSeenIp?: string;
  lastSeenUserAgent?: string;
  lastSeenAt: string;
  syncAll: boolean;
  allowedSystemSlugs?: string[];
  boundAppPasswordId?: string | null;
  boundAppPasswordName?: string;
  boundAppPasswordLastFour?: string;
  lastSyncedAt: string;
  createdAt: string;
};

export type AppPassword = {
  id: string;
  name: string;
  lastFour: string;
  createdAt: string;
  lastUsedAt?: string;
  boundDeviceId?: number | null;
  syncAll: boolean;
  allowedSystemSlugs?: string[];
};

export type AppPasswordAutoEnrollStatus = {
  success?: boolean;
  active: boolean;
  enabledUntil?: string | null;
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
