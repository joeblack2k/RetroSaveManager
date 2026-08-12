import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetchJSON } from "../apiClient";

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" }
  });
}

describe("apiFetchJSON browser write protection", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("adds the CSRF marker to mutating requests", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiFetchJSON("/devices/1", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ alias: "Deck" })
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.credentials).toBe("include");
    expect(init.headers).toMatchObject({
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-CSRF-Protection": "1"
    });
  });

  it("does not add the CSRF marker to safe reads", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiFetchJSON("/auth/me");

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.headers).toMatchObject({ Accept: "application/json" });
    expect((init.headers as Record<string, string>)["X-CSRF-Protection"]).toBeUndefined();
  });

  it("does not let callers remove the required CSRF marker", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiFetchJSON("/save", {
      method: "DELETE",
      headers: { "X-CSRF-Protection": "0" }
    });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect((init.headers as Record<string, string>)["X-CSRF-Protection"]).toBe("1");
  });
});
