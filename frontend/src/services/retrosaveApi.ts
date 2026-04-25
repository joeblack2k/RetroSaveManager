import { apiFetchJSON } from "./apiClient";
import type {
  CheatAdapterDescriptor,
  CheatLibraryStatus,
  CheatManagedPack,
  AppPassword,
  AppPasswordAutoEnrollStatus,
  AuthUser,
  CatalogItem,
  Conflict,
  Device,
  DeviceConfigGlobal,
  DeviceConfigSource,
  GameModuleLibraryStatus,
  GameModuleListResponse,
  GameModuleRecord,
  GameModuleRescanResponse,
  LibraryEntry,
  ReferralInfo,
  RoadmapItem,
  RoadmapSuggestion,
  SaveSystem,
  SaveHistoryResponse,
  SaveCheatResponse,
  SaveSummary,
  SyncLogPage
} from "./types";

export function login(email: string, password: string): Promise<{ success: boolean; message: string }> {
  return apiFetchJSON("/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password, deviceType: "web", fingerprint: "browser" })
  });
}

export function signup(email: string, password: string): Promise<{ success: boolean; message: string }> {
  return apiFetchJSON("/auth/signup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password })
  });
}

export function forgotPassword(email: string): Promise<{ success: boolean; message: string }> {
  return apiFetchJSON("/auth/forgot-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email })
  });
}

export function resetPassword(token: string, newPassword: string): Promise<{ success: boolean; message: string }> {
  return apiFetchJSON("/auth/reset-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, newPassword })
  });
}

export function verifyDevice(userCode: string): Promise<{ success?: boolean; expiresAt?: string }> {
  return apiFetchJSON("/auth/device/verify", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ userCode })
  });
}

export async function getCurrentUser(): Promise<AuthUser> {
  const response = await apiFetchJSON<{ user: AuthUser }>("/auth/me");
  return response.user;
}

export async function listSaves(): Promise<SaveSummary[]> {
  const response = await apiFetchJSON<{ saves: SaveSummary[] }>("/saves?limit=100&offset=0");
  return response.saves;
}

export function uploadSaveFile(params: {
  file: File;
  system?: string;
  slotName?: string;
  romSha1?: string;
  wiiTitleId?: string;
}): Promise<{
  success: boolean;
  save?: { id: string; sha256: string; version?: number };
  successCount?: number;
  errorCount?: number;
  results?: unknown[];
}> {
  const form = new FormData();
  form.append("file", params.file);
  if (params.system) {
    form.append("system", params.system);
  }
  if (params.slotName) {
    form.append("slotName", params.slotName);
  }
  if (params.romSha1) {
    form.append("rom_sha1", params.romSha1);
  }
  if (params.wiiTitleId) {
    form.append("wiiTitleId", params.wiiTitleId);
  }
  return apiFetchJSON("/saves", {
    method: "POST",
    body: form
  });
}

export async function getSaveHistory(params: {
  saveId?: string;
  gameId?: number;
  systemSlug?: string;
  displayTitle?: string;
  psLogicalKey?: string;
}): Promise<SaveHistoryResponse> {
  const search = new URLSearchParams();
  if (params.saveId) {
    search.set("saveId", params.saveId);
  }
  if (typeof params.gameId === "number" && params.gameId > 0) {
    search.set("gameId", String(params.gameId));
  }
  if (params.systemSlug) {
    search.set("systemSlug", params.systemSlug);
  }
  if (params.displayTitle) {
    search.set("displayTitle", params.displayTitle);
  }
  if (params.psLogicalKey) {
    search.set("psLogicalKey", params.psLogicalKey);
  }
  const suffix = search.toString();
  return apiFetchJSON<SaveHistoryResponse>(`/save${suffix ? `?${suffix}` : ""}`);
}

export function rollbackSave(params: {
  saveId: string;
  psLogicalKey?: string;
  revisionId?: string;
}): Promise<{ success: boolean; sourceSaveId: string; save: SaveSummary }> {
  return apiFetchJSON("/save/rollback", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params)
  });
}

export function getSaveCheats(saveId: string, psLogicalKey?: string, saturnEntry?: string): Promise<SaveCheatResponse> {
  const search = new URLSearchParams({ saveId });
  if (psLogicalKey) {
    search.set("psLogicalKey", psLogicalKey);
  }
  if (saturnEntry) {
    search.set("saturnEntry", saturnEntry);
  }
  return apiFetchJSON<SaveCheatResponse>(`/save/cheats?${search.toString()}`);
}

export function applySaveCheats(params: {
  saveId: string;
  psLogicalKey?: string;
  saturnEntry?: string;
  editorId: string;
  slotId?: string;
  updates?: Record<string, unknown>;
  presetIds?: string[];
}): Promise<{ success: boolean; sourceSaveId: string; save: SaveSummary }> {
  return apiFetchJSON("/save/cheats/apply", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params)
  });
}

export async function listCheatPacks(): Promise<CheatManagedPack[]> {
  const response = await apiFetchJSON<{ packs: CheatManagedPack[] }>("/api/cheats/packs");
  return response.packs;
}

export function createCheatPack(params: {
  yaml: string;
  source?: string;
  publishedBy?: string;
  notes?: string;
}): Promise<{ success: boolean; pack: CheatManagedPack }> {
  return apiFetchJSON("/api/cheats/packs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params)
  });
}

export function deleteCheatPack(packId: string): Promise<{ success: boolean; pack: CheatManagedPack }> {
  return apiFetchJSON(`/api/cheats/packs/${encodeURIComponent(packId)}`, {
    method: "DELETE"
  });
}

export function disableCheatPack(packId: string): Promise<{ success: boolean; pack: CheatManagedPack }> {
  return apiFetchJSON(`/api/cheats/packs/${encodeURIComponent(packId)}/disable`, {
    method: "POST"
  });
}

export function enableCheatPack(packId: string): Promise<{ success: boolean; pack: CheatManagedPack }> {
  return apiFetchJSON(`/api/cheats/packs/${encodeURIComponent(packId)}/enable`, {
    method: "POST"
  });
}

export async function listCheatAdapters(): Promise<CheatAdapterDescriptor[]> {
  const response = await apiFetchJSON<{ adapters: CheatAdapterDescriptor[] }>("/api/cheats/adapters");
  return response.adapters;
}

export async function getCheatLibrary(): Promise<CheatLibraryStatus> {
  const response = await apiFetchJSON<{ library: CheatLibraryStatus }>("/api/cheats/library");
  return response.library;
}

export async function syncCheatLibrary(): Promise<CheatLibraryStatus> {
  const response = await apiFetchJSON<{ library: CheatLibraryStatus }>("/api/cheats/library/sync", {
    method: "POST"
  });
  return response.library;
}

export function listGameModules(): Promise<GameModuleListResponse> {
  return apiFetchJSON<GameModuleListResponse>("/api/modules");
}

export async function syncGameModules(): Promise<GameModuleLibraryStatus> {
  const response = await apiFetchJSON<{ library: GameModuleLibraryStatus }>("/api/modules/sync", {
    method: "POST"
  });
  return response.library;
}

export async function uploadGameModule(file: File): Promise<GameModuleRecord> {
  const form = new FormData();
  form.append("file", file);
  const response = await apiFetchJSON<{ module: GameModuleRecord }>("/api/modules/upload", {
    method: "POST",
    body: form
  });
  return response.module;
}

export async function enableGameModule(moduleId: string): Promise<GameModuleRecord> {
  const response = await apiFetchJSON<{ module: GameModuleRecord }>(`/api/modules/${encodeURIComponent(moduleId)}/enable`, {
    method: "POST"
  });
  return response.module;
}

export async function disableGameModule(moduleId: string): Promise<GameModuleRecord> {
  const response = await apiFetchJSON<{ module: GameModuleRecord }>(`/api/modules/${encodeURIComponent(moduleId)}/disable`, {
    method: "POST"
  });
  return response.module;
}

export async function deleteGameModule(moduleId: string): Promise<GameModuleRecord> {
  const response = await apiFetchJSON<{ module: GameModuleRecord }>(`/api/modules/${encodeURIComponent(moduleId)}`, {
    method: "DELETE"
  });
  return response.module;
}

export function rescanGameModules(): Promise<GameModuleRescanResponse> {
  return apiFetchJSON<GameModuleRescanResponse>("/api/modules/rescan", {
    method: "POST"
  });
}

export function deleteSave(id: string, options?: { psLogicalKey?: string }): Promise<{ success: boolean; remainingVersions: number }> {
  const search = new URLSearchParams({ id });
  if (options?.psLogicalKey) {
    search.set("psLogicalKey", options.psLogicalKey);
  }
  return apiFetchJSON(`/save?${search.toString()}`, {
    method: "DELETE"
  });
}

export function deleteManySaves(ids: string[]): Promise<{ success: boolean; remainingVersions: number }> {
  const cleaned = ids.map((id) => id.trim()).filter((id) => id.length > 0);
  if (cleaned.length === 1) {
    return deleteSave(cleaned[0]);
  }
  return apiFetchJSON(`/saves?ids=${encodeURIComponent(cleaned.join(","))}`, {
    method: "DELETE"
  });
}

export async function listCatalog(): Promise<CatalogItem[]> {
  const response = await apiFetchJSON<{ items: CatalogItem[] }>("/catalog");
  return response.items;
}

export async function listLibrary(): Promise<LibraryEntry[]> {
  const response = await apiFetchJSON<{ games: LibraryEntry[] }>("/games/library");
  return response.games;
}

export function addLibraryGame(catalogId: string): Promise<{ success: boolean; entry: LibraryEntry }> {
  return apiFetchJSON("/games/library", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ catalogId })
  });
}

export function removeLibraryGame(id: string): Promise<{ success: boolean }> {
  return apiFetchJSON(`/games/library/${id}`, { method: "DELETE" });
}

export async function listConflicts(): Promise<Conflict[]> {
  const response = await apiFetchJSON<{ conflicts: Conflict[] }>("/conflicts");
  return response.conflicts;
}

export async function listDevices(): Promise<Device[]> {
  const response = await apiFetchJSON<{ devices: Device[] }>("/devices");
  return response.devices;
}

export function listSyncLogs(params?: { hours?: number; page?: number; limit?: number }): Promise<SyncLogPage> {
  const search = new URLSearchParams();
  search.set("hours", String(params?.hours ?? 72));
  search.set("page", String(params?.page ?? 1));
  search.set("limit", String(params?.limit ?? 50));
  return apiFetchJSON<SyncLogPage>(`/logs?${search.toString()}`);
}

export async function getDevice(id: number): Promise<Device> {
  const response = await apiFetchJSON<{ device: Device }>(`/devices/${id}`);
  return response.device;
}

export function updateDevice(
  id: number,
  payload: {
    alias?: string;
    syncAll?: boolean;
    allowedSystemSlugs?: string[];
    configSources?: DeviceConfigSource[];
    configGlobal?: DeviceConfigGlobal;
  }
): Promise<{ success: boolean; device: Device }> {
  return apiFetchJSON(`/devices/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

export function commandDevice(
  id: number,
  command: "sync" | "scan" | "deep_scan",
  reason?: string
): Promise<{ success: boolean; event: string; action: string; broadcast: boolean }> {
  return apiFetchJSON(`/devices/${id}/command`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ command, reason })
  });
}

export function renameDevice(id: number, alias: string): Promise<{ success: boolean; device: Device }> {
  return updateDevice(id, { alias });
}

export function deleteDevice(id: number): Promise<{ success: boolean }> {
  return apiFetchJSON(`/devices/${id}`, { method: "DELETE" });
}

export async function listSaveSystems(): Promise<SaveSystem[]> {
  return apiFetchJSON<SaveSystem[]>("/saves/systems");
}

export async function listAppPasswords(): Promise<AppPassword[]> {
  const response = await apiFetchJSON<{ appPasswords: AppPassword[] }>("/auth/app-passwords");
  return response.appPasswords;
}

export function createAppPassword(name?: string): Promise<{
  success: boolean;
  appPassword: AppPassword;
  plainTextKey: string;
  plainTextToken: string;
}> {
  return apiFetchJSON("/auth/app-passwords", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name })
  });
}

export function revokeAppPassword(id: string): Promise<{ success: boolean }> {
  return apiFetchJSON(`/auth/app-passwords/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
}

export function getAutoAppPasswordEnrollmentStatus(): Promise<AppPasswordAutoEnrollStatus> {
  return apiFetchJSON<AppPasswordAutoEnrollStatus>("/auth/app-passwords/auto-enroll");
}

export function enableAutoAppPasswordEnrollment(minutes = 15): Promise<AppPasswordAutoEnrollStatus> {
  return apiFetchJSON<AppPasswordAutoEnrollStatus>("/auth/app-passwords/auto-enroll", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ minutes })
  });
}

export async function getReferral(): Promise<ReferralInfo> {
  const response = await apiFetchJSON<ReferralInfo>("/referral");
  return {
    code: response.code,
    url: response.url,
    stats: response.stats
  };
}

export async function listRoadmapItems(): Promise<RoadmapItem[]> {
  const response = await apiFetchJSON<{ items: RoadmapItem[] }>("/roadmap/items");
  return response.items;
}

export function voteRoadmapItem(id: string): Promise<{ success: boolean; item: RoadmapItem }> {
  return apiFetchJSON(`/roadmap/items/${id}/vote`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({})
  });
}

export function createRoadmapSuggestion(title: string, description: string): Promise<{ success: boolean; suggestion: RoadmapSuggestion }> {
  return apiFetchJSON("/roadmap/suggestions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, description })
  });
}

export async function listMyRoadmapSuggestions(): Promise<RoadmapSuggestion[]> {
  const response = await apiFetchJSON<{ items: RoadmapSuggestion[] }>("/roadmap/suggestions/mine");
  return response.items;
}
