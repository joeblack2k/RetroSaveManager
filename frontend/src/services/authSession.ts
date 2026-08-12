import { apiFetchJSON } from "./apiClient";

const frontendAuthRequired = String(import.meta.env.VITE_AUTH_REQUIRED ?? "false").trim().toLowerCase() === "true";
const sessionStorageKey = "retrosavemanager.frontend_session";

export function isFrontendAuthRequired(): boolean {
  return frontendAuthRequired;
}

export function hasFrontendAuthSession(): boolean {
  if (!frontendAuthRequired) {
    return true;
  }
  if (typeof window === "undefined") {
    return false;
  }
  return window.localStorage.getItem(sessionStorageKey) === "1";
}

export function markFrontendAuthSession(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(sessionStorageKey, "1");
}

export function clearFrontendAuthSession(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(sessionStorageKey);
}

// Logout is best-effort from the UI's perspective. The backend call is needed
// to expire its HttpOnly cookie, while local state must still be removed when
// the server is temporarily unavailable.
export async function logoutFrontendAuthSession(): Promise<void> {
  try {
    await apiFetchJSON<{ success: boolean; message?: string }>("/auth/logout", {
      method: "POST"
    });
  } catch {
    // The local session is cleared below so the user is never trapped in the
    // authenticated UI because the server or network is unavailable.
  } finally {
    clearFrontendAuthSession();
  }
}
