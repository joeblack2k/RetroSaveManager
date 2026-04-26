import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeviceManagePage } from "../DeviceManagePage";
import { updateDevice } from "../../../services/retrosaveApi";

vi.mock("../../../services/retrosaveApi", () => ({
  commandDevice: vi.fn(() => Promise.resolve({ success: true, event: "sync.requested", action: "sync", broadcast: true })),
  getDevice: vi.fn(() =>
    Promise.resolve({
      id: 42,
      deviceType: "linux-x86",
      fingerprint: "deck-001",
      alias: null,
      displayName: "Steam Deck",
      hostname: "steamdeck.example.invalid",
      helperName: "RSM Helper",
      helperVersion: "0.4.14",
      platform: "linux/x86_64",
      syncPaths: [],
      reportedSystemSlugs: [],
      configSources: [],
      effectivePolicy: undefined,
      lastSeenAt: "2026-04-25T12:00:00Z",
      lastSyncedAt: "2026-04-25T12:00:00Z",
      syncAll: true,
      allowedSystemSlugs: [],
      createdAt: "2026-04-25T12:00:00Z"
    })
  ),
  listSaveSystems: vi.fn(() =>
    Promise.resolve([
      { id: 1, name: "Super Nintendo", slug: "snes", manufacturer: "Nintendo" },
      { id: 2, name: "Nintendo 64", slug: "n64", manufacturer: "Nintendo" }
    ])
  ),
  updateDevice: vi.fn(() => Promise.resolve({ success: true, device: {} }))
}));

describe("DeviceManagePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("adds a backend-managed console source to the device policy", async () => {
    render(
      <MemoryRouter initialEntries={["/app/devices/42/manage"]}>
        <Routes>
          <Route path="/app/devices/:deviceId/manage" element={<DeviceManagePage />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText("Steam Deck")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Profile"), { target: { value: "snes9x" } });
    fireEvent.change(screen.getByLabelText("Save folder"), { target: { value: "/media/snes9x/saves" } });
    fireEvent.change(screen.getByLabelText("ROM folder"), { target: { value: "/media/snes9x/roms" } });
    fireEvent.click(screen.getByRole("button", { name: "Add console" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(updateDevice).toHaveBeenCalledWith(
        42,
        expect.objectContaining({
          configSources: [
            expect.objectContaining({
              id: "backend-snes-snes9x",
              profile: "snes9x",
              savePaths: ["/media/snes9x/saves"],
              romPaths: ["/media/snes9x/roms"],
              systems: ["snes"],
              managed: true,
              origin: "backend"
            })
          ]
        })
      );
    });
  });
});
