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
