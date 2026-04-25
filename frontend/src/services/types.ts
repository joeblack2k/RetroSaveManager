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

export type SaveCheatCapability = {
  supported: boolean;
  availableCount?: number;
  editorId?: string;
  adapterId?: string;
  packId?: string;
};

export type SaveCheatOption = {
  id: string;
  label: string;
};

export type SaveCheatBitOption = {
  id: string;
  bit: number;
  label: string;
};

export type SaveCheatField = {
  id: string;
  ref?: string;
  label: string;
  description?: string;
  type: "boolean" | "integer" | "enum" | "bitmask" | string;
  min?: number;
  max?: number;
  step?: number;
  options?: SaveCheatOption[];
  bits?: SaveCheatBitOption[];
};

export type SaveCheatSection = {
  id: string;
  title: string;
  fields: SaveCheatField[];
};

export type SaveCheatPreset = {
  id: string;
  label: string;
  description?: string;
  updates?: Record<string, unknown>;
};

export type SaveCheatSelector = {
  id: string;
  label: string;
  type: string;
  options?: SaveCheatOption[];
};

export type SaveCheatEditorState = {
  supported: boolean;
  gameId?: string;
  systemSlug?: string;
  editorId?: string;
  adapterId?: string;
  packId?: string;
  title?: string;
  availableCount?: number;
  selector?: SaveCheatSelector | null;
  sections?: SaveCheatSection[];
  presets?: SaveCheatPreset[];
  values?: Record<string, unknown>;
  slotValues?: Record<string, Record<string, unknown>>;
};

export type SaveCheatResponse = {
  success: boolean;
  saveId: string;
  displayTitle: string;
  cheats: SaveCheatEditorState;
};

export type CheatPack = {
  packId?: string;
  schemaVersion?: number;
  adapterId?: string;
  gameId?: string;
  systemSlug?: string;
  editorId?: string;
  title?: string;
  match?: { titleAliases?: string[] };
  selector?: SaveCheatSelector | null;
  sections?: SaveCheatSection[];
  presets?: SaveCheatPreset[];
};

export type CheatPackManifest = {
  packId: string;
  adapterId: string;
  source: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  publishedBy?: string;
  notes?: string;
  sourcePath?: string;
  sourceRevision?: string;
  sourceSha256?: string;
  lastSyncedAt?: string;
};

export type CheatManagedPack = {
  pack: CheatPack;
  manifest: CheatPackManifest;
  builtin: boolean;
  supportsSaveUi: boolean;
};

export type CheatAdapterDescriptor = {
  id: string;
  kind: string;
  family: string;
  systemSlug: string;
  requiredParserId?: string;
  minimumParserLevel?: string;
  supportsRuntimeProfiles: boolean;
  supportsLogicalSaves: boolean;
  supportsLiveUpload: boolean;
  matchKeys?: string[];
};

export type CheatLibraryConfig = {
  repo: string;
  ref: string;
  path: string;
};

export type CheatLibraryImportedPack = {
  path: string;
  packId?: string;
  title?: string;
  systemSlug?: string;
  sourceSha256?: string;
  status?: string;
};

export type CheatLibrarySyncError = {
  path: string;
  message: string;
};

export type CheatLibraryStatus = {
  config: CheatLibraryConfig;
  lastSyncedAt?: string;
  importedCount: number;
  errorCount: number;
  imported: CheatLibraryImportedPack[];
  errors: CheatLibrarySyncError[];
};

export type GameModuleCheatPackRef = {
  path: string;
};

export type GameModulePayloadPolicy = {
  exactSizes?: number[];
  formats?: string[];
};

export type GameModuleManifest = {
  moduleId: string;
  schemaVersion: number;
  version: string;
  systemSlug: string;
  gameId: string;
  title: string;
  parserId: string;
  wasmFile: string;
  abiVersion: string;
  cheatPacks?: GameModuleCheatPackRef[];
  payload: GameModulePayloadPolicy;
  titleAliases?: string[];
  romHashes?: string[];
};

export type GameModuleRecord = {
  manifest: GameModuleManifest;
  status: string;
  source: string;
  sourcePath?: string;
  sourceRevision?: string;
  sourceSha256?: string;
  importedAt: string;
  updatedAt: string;
  lastSyncedAt?: string;
  errors?: string[];
  cheatPackIds?: string[];
};

export type GameModuleLibraryConfig = {
  repo: string;
  ref: string;
  path: string;
};

export type GameModuleSyncImported = {
  path: string;
  moduleId?: string;
  title?: string;
  systemSlug?: string;
  status?: string;
  sha256?: string;
};

export type GameModuleSyncError = {
  path: string;
  message: string;
};

export type GameModuleLibraryStatus = {
  config: GameModuleLibraryConfig;
  lastSyncedAt?: string;
  importedCount: number;
  errorCount: number;
  imported: GameModuleSyncImported[];
  errors: GameModuleSyncError[];
};

export type GameModuleListResponse = {
  success: boolean;
  modules: GameModuleRecord[];
  library: GameModuleLibraryStatus;
};

export type GameModuleRescanResponse = {
  success: boolean;
  result: {
    scanned?: number;
    accepted?: number;
    rejected?: number;
    updated?: number;
    [key: string]: unknown;
  };
};

export type SaveDownloadProfile = {
  id: string;
  label: string;
  targetExtension?: string;
  note?: string;
};

export type SaveInspection = {
  parserLevel?: string;
  parserId?: string;
  validatedSystem?: string;
  validatedGameId?: string;
  validatedGameTitle?: string;
  trustLevel?: string;
  evidence?: string[];
  warnings?: string[];
  payloadSizeBytes?: number;
  slotCount?: number;
  activeSlotIndexes?: number[];
  checksumValid?: boolean;
  semanticFields?: Record<string, unknown>;
};

export type SaveSummary = {
  id: string;
  game: SaveGame;
  cheats?: SaveCheatCapability | null;
  downloadProfiles?: SaveDownloadProfile[];
  displayTitle?: string;
  logicalKey?: string;
  systemSlug?: string;
  regionCode?: "US" | "EU" | "JP" | "UNKNOWN" | string;
  regionFlag?: string;
  languageCodes?: string[];
  coverArtUrl?: string;
  mediaType?: string;
  projectionCapable?: boolean;
  sourceArtifactProfile?: string;
  saveCount?: number;
  latestSizeBytes?: number;
  totalSizeBytes?: number;
  latestVersion?: number;
  memoryCard?: MemoryCardDetails | null;
  dreamcast?: Record<string, unknown> | null;
  saturn?: Record<string, unknown> | null;
  inspection?: SaveInspection | null;
  runtimeProfile?: string;
  cardSlot?: string;
  projectionId?: string;
  sourceImportId?: string;
  portable?: boolean;
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
  configRevision?: string;
  configReportedAt?: string;
  configGlobal?: DeviceConfigGlobal;
  configSources?: DeviceConfigSource[];
  configCapabilities?: Record<string, unknown>;
  service?: DeviceServiceState;
  sensors?: DeviceSensorState;
  effectivePolicy?: DeviceEffectivePolicy;
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

export type DeviceServiceState = {
  mode?: string;
  status?: string;
  loop?: string;
  controlChannel?: string;
  heartbeatInterval?: number;
  reconcileInterval?: number;
  pid?: number;
  startedAt?: string;
  uptimeSeconds?: number;
  binaryPath?: string;
  lastSyncStartedAt?: string;
  lastSyncFinishedAt?: string;
  lastSyncOk?: boolean;
  lastError?: string;
  lastEvent?: string;
  syncCycles?: number;
  lastHeartbeatAt?: string;
  online?: boolean;
  freshness?: "online" | "degraded" | "stale" | "offline" | string;
  staleAfterSeconds?: number;
  offlineAfterSeconds?: number;
  offlineAt?: string;
};

export type DeviceLastSyncStats = {
  scanned: number;
  uploaded: number;
  downloaded: number;
  inSync: number;
  conflicts: number;
  skipped: number;
  errors: number;
};

export type DeviceSensorState = {
  online?: boolean;
  authenticated?: boolean;
  configHash?: string;
  configReadable?: boolean;
  configError?: string;
  sourceCount?: number;
  savePathCount?: number;
  romPathCount?: number;
  configuredSystems?: string[];
  supportedSystems?: string[];
  syncLockPresent?: boolean;
  lastSync?: DeviceLastSyncStats;
};

export type DeviceConfigGlobal = {
  url?: string;
  port?: number;
  baseUrl?: string;
  email?: string;
  appPasswordConfigured?: boolean;
  root?: string;
  stateDir?: string;
  watch?: boolean;
  watchInterval?: number;
  forceUpload?: boolean;
  dryRun?: boolean;
  routePrefix?: string;
};

export type DeviceConfigSource = {
  id: string;
  label?: string;
  kind?: string;
  profile?: string;
  savePath?: string;
  savePaths?: string[];
  romPath?: string;
  romPaths?: string[];
  recursive?: boolean;
  systems?: string[];
  unsupportedSystemSlugs?: string[];
  createMissingSystemDirs?: boolean;
  managed?: boolean;
  origin?: string;
};

export type DevicePolicyBlock = {
  system: string;
  reason: string;
  sourceId?: string;
  sourceLabel?: string;
};

export type DeviceSourceEffectivePolicy = {
  sourceId?: string;
  sourceLabel?: string;
  kind?: string;
  profile?: string;
  allowedSystemSlugs?: string[];
  blocked?: DevicePolicyBlock[];
};

export type DeviceEffectivePolicy = {
  mode: string;
  allowedSystemSlugs?: string[];
  blocked?: DevicePolicyBlock[];
  sources?: DeviceSourceEffectivePolicy[];
};

export type SyncLogEntry = {
  id: string;
  createdAt: string;
  deviceName: string;
  action: string;
  game: string;
  error: boolean;
  errorMessage?: string;
  systemSlug?: string;
  saveId?: string;
  conflictId?: string;
};

export type SyncLogPage = {
  success?: boolean;
  generatedAt?: string;
  hours: number;
  page: number;
  limit: number;
  total: number;
  totalPages: number;
  logs: SyncLogEntry[];
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
