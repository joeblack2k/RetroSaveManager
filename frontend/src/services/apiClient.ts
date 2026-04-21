const configuredBase = (import.meta.env.VITE_API_BASE_URL ?? "").trim();
const API_BASE = configuredBase.replace(/\/$/, "");

function toAbsolutePath(path: string): string {
  if (/^https?:\/\//.test(path)) {
    return path;
  }
  const normalized = path.startsWith("/") ? path : `/${path}`;
  if (!API_BASE) {
    return normalized;
  }
  return `${API_BASE}${normalized}`;
}

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export async function apiFetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(toAbsolutePath(path), {
    credentials: "include",
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (!response.ok) {
    const fallback = `${response.status} ${response.statusText}`.trim();
    let message = fallback;
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      const body = (await response.json()) as { message?: string; error?: string };
      message = body.message || body.error || fallback;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return (await response.json()) as T;
}

export function apiDownloadURL(path: string): string {
  return toAbsolutePath(path);
}

export function apiBaseForUi(): string {
  if (API_BASE) {
    return API_BASE;
  }
  if (typeof window !== "undefined") {
    return window.location.origin;
  }
  return "";
}
