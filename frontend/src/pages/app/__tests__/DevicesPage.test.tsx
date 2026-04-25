import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DevicesPage } from "../DevicesPage";
import { commandDevice, deleteDevice } from "../../../services/retrosaveApi";

vi.mock("../../../services/retrosaveApi", () => ({
  commandDevice: vi.fn(() => Promise.resolve({ success: true, event: "sync.requested", action: "sync", broadcast: true })),
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
        configRevision: "sha256:example",
        configReportedAt: "2026-04-23T12:29:00Z",
        configGlobal: {
          baseUrl: "https://rsm.example.invalid",
          root: "/media/fat",
          stateDir: "./state",
          watch: true,
          watchInterval: 30,
          forceUpload: false,
          dryRun: false,
          appPasswordConfigured: true
        },
        configSources: [
          {
            id: "mister_default",
            label: "MiSTer Default",
            kind: "mister-fpga",
            profile: "mister",
            savePath: "/media/fat/saves",
            romPath: "/media/fat/games",
            systems: ["snes", "psx"],
            unsupportedSystemSlugs: ["gbc"],
            managed: false,
            origin: "manual"
          }
        ],
        service: {
          mode: "daemon",
          status: "idle",
          loop: "sse-plus-periodic-reconcile",
          controlChannel: "GET /events",
          heartbeatInterval: 30,
          reconcileInterval: 1800,
          lastHeartbeatAt: "2026-04-23T12:31:30Z",
          lastEvent: "startup",
          syncCycles: 3,
          lastSyncOk: true,
          online: true,
          freshness: "online"
        },
        sensors: {
          online: true,
          authenticated: true,
          configHash: "sha256-config",
          configReadable: true,
          sourceCount: 1,
          savePathCount: 1,
          romPathCount: 1,
          configuredSystems: ["snes", "psx"],
          supportedSystems: ["snes", "psx", "n64"],
          syncLockPresent: false,
          lastSync: { scanned: 12, uploaded: 1, downloaded: 2, inSync: 9, conflicts: 0, skipped: 0, errors: 0 }
        },
        effectivePolicy: {
          mode: "manual-source-intersection",
          allowedSystemSlugs: ["snes", "psx"],
          blocked: [{ system: "wii", reason: "not supported by this helper kind/profile", sourceId: "mister_default", sourceLabel: "MiSTer Default" }]
        },
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

  it("renders a compact device overview and opens rich details in a modal", async () => {
    render(
      <MemoryRouter>
        <DevicesPage />
      </MemoryRouter>
    );

    expect(await screen.findByText("Living Room MiSTer")).toBeInTheDocument();
    expect(screen.getByLabelText("Device fleet summary")).toBeInTheDocument();
    expect(screen.getByText(/mister-01\.example\.invalid/)).toBeInTheDocument();
    expect(screen.getByText(/198\.51\.100\.42/)).toBeInTheDocument();
    expect(screen.getByText(/RSM Helper/)).toBeInTheDocument();
    expect(screen.getByText(/2.1.0/)).toBeInTheDocument();
    expect(screen.getAllByText("online").length).toBeGreaterThan(0);
    expect(screen.getByText("1 up / 2 down")).toBeInTheDocument();
    expect(screen.queryByText("sha256-config")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Details" }));

    expect(screen.getByText(/sse-plus-periodic-reconcile/)).toBeInTheDocument();
    expect(screen.getByText("1800s")).toBeInTheDocument();
    expect(screen.getByText("sha256-config")).toBeInTheDocument();
    expect(screen.getByText("12 scanned, 1 uploaded, 2 downloaded, 0 errors")).toBeInTheDocument();
    expect(screen.getByText("https://rsm.example.invalid")).toBeInTheDocument();
    expect(screen.getByText("/media/fat")).toBeInTheDocument();
    expect(screen.getAllByText("30s").length).toBeGreaterThan(0);
    expect(screen.getByText("/media/fat/saves/SNES")).toBeInTheDocument();
    expect(screen.getByText("/media/fat/saves/PSX")).toBeInTheDocument();
    expect(screen.getAllByText("snes").length).toBeGreaterThan(0);
    expect(screen.getAllByText("psx").length).toBeGreaterThan(0);
    expect(screen.getByText("Helper config sources")).toBeInTheDocument();
    expect(screen.getByText("MiSTer Default")).toBeInTheDocument();
    expect(screen.getByText("/media/fat/games")).toBeInTheDocument();
    expect(screen.getByText(/sha256:example/)).toBeInTheDocument();
    expect(screen.getByText("wii")).toBeInTheDocument();
  });

  it("publishes a helper sync command", async () => {
    render(
      <MemoryRouter>
        <DevicesPage />
      </MemoryRouter>
    );

    const button = await screen.findByRole("button", { name: "Sync" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(commandDevice).toHaveBeenCalledWith(42, "sync", "devices_page");
    });
  });

  it("deletes a device after confirmation", async () => {
    render(
      <MemoryRouter>
        <DevicesPage />
      </MemoryRouter>
    );

    fireEvent.click(await screen.findByRole("button", { name: "Details" }));

    const button = await screen.findByRole("button", { name: "Delete" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(deleteDevice).toHaveBeenCalledWith(42);
    });
  });
});
