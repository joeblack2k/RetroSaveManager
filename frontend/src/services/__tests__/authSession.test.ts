import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiFetchJSON } from "../apiClient";
import { logoutFrontendAuthSession, markFrontendAuthSession } from "../authSession";

vi.mock("../apiClient", () => ({
  apiFetchJSON: vi.fn()
}));

const sessionStorageKey = "retrosavemanager.frontend_session";

describe("logoutFrontendAuthSession", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.clearAllMocks();
  });

  it("posts to the backend logout endpoint and clears local state", async () => {
    vi.mocked(apiFetchJSON).mockResolvedValue({ success: true, message: "Logged out" });
    markFrontendAuthSession();

    await logoutFrontendAuthSession();

    expect(apiFetchJSON).toHaveBeenCalledWith("/auth/logout", { method: "POST" });
    expect(window.localStorage.getItem(sessionStorageKey)).toBeNull();
  });

  it("still clears local state when the backend is unavailable", async () => {
    vi.mocked(apiFetchJSON).mockRejectedValue(new Error("offline"));
    markFrontendAuthSession();

    await expect(logoutFrontendAuthSession()).resolves.toBeUndefined();

    expect(window.localStorage.getItem(sessionStorageKey)).toBeNull();
  });
});
