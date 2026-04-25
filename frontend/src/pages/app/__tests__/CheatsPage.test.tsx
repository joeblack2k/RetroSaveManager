import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CheatsPage } from "../CheatsPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { CheatAdapterDescriptor, CheatLibraryStatus, CheatManagedPack } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  listCheatPacks: vi.fn(),
  listCheatAdapters: vi.fn(),
  getCheatLibrary: vi.fn(),
  syncCheatLibrary: vi.fn(),
  createCheatPack: vi.fn()
}));

describe("CheatsPage", () => {
  const adapters: CheatAdapterDescriptor[] = [
    {
      id: "sm64-eeprom",
      kind: "legacy",
      family: "sm64-eeprom",
      systemSlug: "n64",
      requiredParserId: "",
      minimumParserLevel: "container",
      supportsRuntimeProfiles: false,
      supportsLogicalSaves: false,
      supportsLiveUpload: true,
      matchKeys: ["systemSlug", "titleAliases"]
    }
  ];

  const libraryStatus: CheatLibraryStatus = {
    config: {
      repo: "joeblack2k/RetroSaveManager",
      ref: "main",
      path: "cheats/packs"
    },
    lastSyncedAt: "2026-04-24T10:00:00Z",
    importedCount: 1,
    errorCount: 0,
    imported: [
      {
        path: "cheats/packs/n64/super-mario-64.yaml",
        packId: "sm64-runtime-ui",
        title: "SM64 Runtime UI",
        systemSlug: "n64",
        status: "active"
      }
    ],
    errors: []
  };

  const activePack: CheatManagedPack = {
    pack: {
      packId: "sm64-runtime-ui",
      schemaVersion: 1,
      adapterId: "sm64-eeprom",
      gameId: "n64/super-mario-64",
      systemSlug: "n64",
      title: "SM64 Runtime UI"
    },
    manifest: {
      packId: "sm64-runtime-ui",
      adapterId: "sm64-eeprom",
      source: "github",
      status: "active",
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z",
      publishedBy: "GitHub library",
      notes: "Initial runtime pack",
      sourcePath: "cheats/packs/n64/super-mario-64.yaml",
      sourceRevision: "main",
      sourceSha256: "abc123",
      lastSyncedAt: "2026-04-24T10:00:00Z"
    },
    builtin: false,
    supportsSaveUi: true
  };

  beforeEach(() => {
    vi.mocked(retrosaveApi.listCheatAdapters).mockResolvedValue(adapters);
    vi.mocked(retrosaveApi.listCheatPacks).mockResolvedValue([activePack]);
    vi.mocked(retrosaveApi.getCheatLibrary).mockResolvedValue(libraryStatus);
    vi.mocked(retrosaveApi.syncCheatLibrary).mockResolvedValue(libraryStatus);
    vi.mocked(retrosaveApi.createCheatPack).mockResolvedValue({ success: true, pack: activePack });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
  });

  it("renders the simple cheat library and keeps adapter details in advanced tools", async () => {
    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole("heading", { name: "Cheat Library" })).toBeInTheDocument();
    expect(await screen.findByText("SM64 Runtime UI")).toBeInTheDocument();
    expect(screen.getAllByText("1 packs").length).toBeGreaterThan(0);
    expect(screen.getByText("1 active")).toBeInTheDocument();
    expect(screen.getByText("joeblack2k/RetroSaveManager@main")).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Actions" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /disable sm64-runtime-ui/i })).not.toBeInTheDocument();
    expect(screen.queryByText("Match keys: systemSlug, titleAliases")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Show advanced tools" }));
    expect(screen.getByText("Match keys: systemSlug, titleAliases")).toBeInTheDocument();
  });

  it("syncs packs from GitHub and refreshes the library", async () => {
    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    await screen.findByRole("heading", { name: "Cheat Library" });
    fireEvent.click(await screen.findByRole("button", { name: "Sync from GitHub" }));

    await waitFor(() => {
      expect(retrosaveApi.syncCheatLibrary).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByText("GitHub sync finished: 1 imported, 0 validation errors.")).toBeInTheDocument();
    expect(retrosaveApi.listCheatPacks).toHaveBeenCalledTimes(2);
  });

  it("shows validation errors without opening advanced tools", async () => {
    vi.mocked(retrosaveApi.getCheatLibrary).mockResolvedValue({
      ...libraryStatus,
      errorCount: 1,
      errors: [{ path: "cheats/packs/n64/broken.yaml", message: "unknown field ref doesNotExist" }]
    });

    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole("heading", { name: "Validation Errors" })).toBeInTheDocument();
    expect(screen.getByText("cheats/packs/n64/broken.yaml")).toBeInTheDocument();
    expect(screen.getByText("unknown field ref doesNotExist")).toBeInTheDocument();
  });

  it("publishes YAML packs from advanced tools and refreshes the list", async () => {
    vi.mocked(retrosaveApi.listCheatPacks).mockResolvedValueOnce([]).mockResolvedValueOnce([activePack]);

    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    await screen.findByRole("heading", { name: "Cheat Library" });
    fireEvent.click(await screen.findByRole("button", { name: "Show advanced tools" }));

    fireEvent.change(screen.getByRole("textbox", { name: "Pack YAML" }), {
      target: {
        value: `packId: sm64-runtime-ui
schemaVersion: 1
adapterId: sm64-eeprom
gameId: n64/super-mario-64
systemSlug: n64
title: SM64 Runtime UI
sections:
  - id: runtime-abilities
    title: Runtime Abilities
    fields:
      - id: runtimeWingCap
        ref: haveWingCap
        label: Runtime Wing Cap
        type: boolean`
      }
    });

    fireEvent.click(screen.getByRole("button", { name: "Publish pack" }));

    await waitFor(() => {
      expect(retrosaveApi.createCheatPack).toHaveBeenCalledWith({
        yaml: expect.stringContaining("packId: sm64-runtime-ui"),
        source: "uploaded",
        publishedBy: undefined,
        notes: undefined
      });
    });

    expect(await screen.findByText("Pack sm64-runtime-ui is now live.")).toBeInTheDocument();
    expect(await screen.findByText("SM64 Runtime UI")).toBeInTheDocument();
  });

});
