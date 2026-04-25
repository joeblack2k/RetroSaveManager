import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsPage } from "../SettingsPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { AppPassword, GameModuleListResponse, GameModuleRecord } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  createAppPassword: vi.fn(),
  deleteGameModule: vi.fn(),
  disableGameModule: vi.fn(),
  enableGameModule: vi.fn(),
  listAppPasswords: vi.fn(),
  listGameModules: vi.fn(),
  rescanGameModules: vi.fn(),
  revokeAppPassword: vi.fn(),
  syncGameModules: vi.fn(),
  uploadGameModule: vi.fn()
}));

describe("SettingsPage", () => {
  const appPasswords: AppPassword[] = [
    {
      id: "key-1",
      name: "MiSTer",
      lastFour: "1234",
      createdAt: "2026-04-25T08:00:00Z",
      syncAll: true
    }
  ];

  const moduleRecord: GameModuleRecord = {
    manifest: {
      moduleId: "gb-module-game",
      schemaVersion: 1,
      version: "1.0.0",
      systemSlug: "gameboy",
      gameId: "gameboy/module-game",
      title: "Module Game",
      parserId: "module-game-parser",
      wasmFile: "parser.wasm",
      abiVersion: "rsm-wasm-json-v1",
      cheatPacks: [{ path: "cheats/module-game.yaml" }],
      payload: { exactSizes: [4], formats: ["sav"] },
      titleAliases: ["Module Game"]
    },
    status: "active",
    source: "github",
    sourcePath: "modules/gameboy/module-game.rsmodule.zip",
    sourceRevision: "main",
    sourceSha256: "abc123",
    importedAt: "2026-04-25T09:00:00Z",
    updatedAt: "2026-04-25T09:00:00Z",
    lastSyncedAt: "2026-04-25T09:00:00Z",
    cheatPackIds: ["gameboy--module-game"]
  };

  const moduleList: GameModuleListResponse = {
    success: true,
    modules: [moduleRecord],
    library: {
      config: {
        repo: "joeblack2k/RetroSaveManager",
        ref: "main",
        path: "modules"
      },
      lastSyncedAt: "2026-04-25T09:00:00Z",
      importedCount: 1,
      errorCount: 0,
      imported: [
        {
          path: "modules/gameboy/module-game.rsmodule.zip",
          moduleId: "gb-module-game",
          title: "Module Game",
          systemSlug: "gameboy",
          status: "active",
          sha256: "abc123"
        }
      ],
      errors: []
    }
  };

  beforeEach(() => {
    vi.mocked(retrosaveApi.listAppPasswords).mockResolvedValue(appPasswords);
    vi.mocked(retrosaveApi.listGameModules).mockResolvedValue(moduleList);
    vi.mocked(retrosaveApi.syncGameModules).mockResolvedValue(moduleList.library);
    vi.mocked(retrosaveApi.rescanGameModules).mockResolvedValue({ success: true, result: { scanned: 4 } });
    vi.mocked(retrosaveApi.disableGameModule).mockResolvedValue({ ...moduleRecord, status: "disabled" });
    vi.mocked(retrosaveApi.enableGameModule).mockResolvedValue(moduleRecord);
    vi.mocked(retrosaveApi.deleteGameModule).mockResolvedValue({ ...moduleRecord, status: "deleted" });
    vi.mocked(retrosaveApi.uploadGameModule).mockResolvedValue(moduleRecord);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders game support modules in settings", async () => {
    render(<SettingsPage />);

    expect(await screen.findByRole("heading", { name: "Game Support Modules" })).toBeInTheDocument();
    expect(screen.getByText("Module Game")).toBeInTheDocument();
    expect(screen.getByText("gb-module-game")).toBeInTheDocument();
    expect(screen.getByText("gameboy - 1 cheat packs")).toBeInTheDocument();
    expect(screen.getByText("1 saved keys")).toBeInTheDocument();
    expect(screen.getByText("Advanced module upload")).toBeInTheDocument();
    expect(screen.queryByText("module-game-parser")).not.toBeInTheDocument();
  });

  it("syncs modules from GitHub and refreshes settings", async () => {
    render(<SettingsPage />);

    await screen.findByRole("heading", { name: "Game Support Modules" });
    fireEvent.click(screen.getByRole("button", { name: "Sync from GitHub" }));

    await waitFor(() => {
      expect(retrosaveApi.syncGameModules).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByText("Module sync finished: 1 imported, 0 validation errors.")).toBeInTheDocument();
    await waitFor(() => {
      expect(retrosaveApi.listGameModules).toHaveBeenCalledTimes(2);
    });
  });

  it("can disable an active module", async () => {
    render(<SettingsPage />);

    await screen.findByRole("heading", { name: "Game Support Modules" });
    fireEvent.click(screen.getByRole("button", { name: "Disable" }));

    await waitFor(() => {
      expect(retrosaveApi.disableGameModule).toHaveBeenCalledWith("gb-module-game");
    });
    expect(await screen.findByText("Module gb-module-game is disabled.")).toBeInTheDocument();
  });
});
