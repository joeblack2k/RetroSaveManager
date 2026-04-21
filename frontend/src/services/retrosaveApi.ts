import { apiFetchJSON } from "./apiClient";
import type {
  AuthUser,
  CatalogItem,
  Conflict,
  Device,
  LibraryEntry,
  ReferralInfo,
  RoadmapItem,
  RoadmapSuggestion,
  SaveSummary
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

export function renameDevice(id: number, alias: string): Promise<{ success: boolean; device: Device }> {
  return apiFetchJSON(`/devices/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ alias })
  });
}

export function deleteDevice(id: number): Promise<{ success: boolean }> {
  return apiFetchJSON(`/devices/${id}`, { method: "DELETE" });
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
