import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DevicesPage } from "../DevicesPage";
import { deleteDevice } from "../../../services/retrosaveApi";

vi.mock("../../../services/retrosaveApi", () => ({
  deleteDevice: vi.fn(() => Promise.resolve({ success: true })),
  listDevices: vi.fn(() =>
    Promise.resolve([
      {
        id: 42,
        deviceType: "mister",
        fingerprint: "mister-001",
        alias: null,
        displayName: "Living Room MiSTer",
        hostname: "mister-01.example.invalid",
        helperName: "RSM Helper",
        helperVersion: "2.1.0",
        platform: "MiSTer",
        syncPaths: ["/media/fat/saves/SNES", "/media/fat/saves/PSX"],
        reportedSystemSlugs: ["snes", "psx"],
        lastSeenIp: "198.51.100.42",
        lastSeenUserAgent: "RSMHelper/2.1.0",
        lastSeenAt: "2026-04-23T12:30:00Z",
        syncAll: false,
        allowedSystemSlugs: ["snes", "psx"],
        boundAppPasswordId: "app-password-1",
        boundAppPasswordName: "Living Room MiSTer",
        boundAppPasswordLastFour: "K9P2",
        lastSyncedAt: "2026-04-23T12:31:00Z",
        createdAt: "2026-04-23T12:00:00Z"
      }
    ])
  )
}));

describe("DevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders rich helper metadata for each device", async () => {
    render(
      <MemoryRouter>
        <DevicesPage />
      </MemoryRouter>
    );

    expect(await screen.findByText("Living Room MiSTer")).toBeInTheDocument();
    expect(screen.getByText("mister-01.example.invalid")).toBeInTheDocument();
    expect(screen.getByText("198.51.100.42")).toBeInTheDocument();
    expect(screen.getByText("RSM Helper")).toBeInTheDocument();
    expect(screen.getByText("2.1.0")).toBeInTheDocument();
    expect(screen.getByText("/media/fat/saves/SNES")).toBeInTheDocument();
    expect(screen.getByText("/media/fat/saves/PSX")).toBeInTheDocument();
    expect(screen.getByText("snes")).toBeInTheDocument();
    expect(screen.getByText("psx")).toBeInTheDocument();
  });

  it("deletes a device after confirmation", async () => {
    render(
      <MemoryRouter>
        <DevicesPage />
      </MemoryRouter>
    );

    const button = await screen.findByRole("button", { name: "Delete" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(deleteDevice).toHaveBeenCalledWith(42);
    });
  });
});
