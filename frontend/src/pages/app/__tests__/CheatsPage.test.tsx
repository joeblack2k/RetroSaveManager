import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CheatsPage } from "../CheatsPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { CheatAdapterDescriptor, CheatManagedPack } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  listCheatPacks: vi.fn(),
  listCheatAdapters: vi.fn(),
  createCheatPack: vi.fn(),
  deleteCheatPack: vi.fn(),
  disableCheatPack: vi.fn(),
  enableCheatPack: vi.fn()
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
      source: "uploaded",
      status: "active",
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z",
      publishedBy: "codex",
      notes: "Initial runtime pack"
    },
    builtin: false,
    supportsSaveUi: true
  };

  beforeEach(() => {
    vi.mocked(retrosaveApi.listCheatAdapters).mockResolvedValue(adapters);
    vi.mocked(retrosaveApi.listCheatPacks).mockResolvedValue([activePack]);
    vi.mocked(retrosaveApi.createCheatPack).mockResolvedValue({ success: true, pack: activePack });
    vi.mocked(retrosaveApi.disableCheatPack).mockResolvedValue({
      success: true,
      pack: {
        ...activePack,
        manifest: { ...activePack.manifest, status: "disabled" },
        supportsSaveUi: false
      }
    });
    vi.mocked(retrosaveApi.enableCheatPack).mockResolvedValue({ success: true, pack: activePack });
    vi.mocked(retrosaveApi.deleteCheatPack).mockResolvedValue({
      success: true,
      pack: {
        ...activePack,
        manifest: { ...activePack.manifest, status: "deleted" },
        supportsSaveUi: false
      }
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
  });

  it("renders pack management and adapter catalog data", async () => {
    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    expect(await screen.findByRole("heading", { name: "Cheats" })).toBeInTheDocument();
    expect(screen.getByText("SM64 Runtime UI")).toBeInTheDocument();
    expect(screen.getByText("Match keys: systemSlug, titleAliases")).toBeInTheDocument();
    expect(screen.getByText("1 packs")).toBeInTheDocument();
    expect(screen.getByText("1 adapters")).toBeInTheDocument();
  });

  it("publishes YAML packs and refreshes the list", async () => {
    vi.mocked(retrosaveApi.listCheatPacks)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([activePack]);

    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    await screen.findByRole("heading", { name: "Cheats" });

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

  it("runs disable, enable, and delete actions and reloads the page data", async () => {
    const disabledPack: CheatManagedPack = {
      ...activePack,
      manifest: { ...activePack.manifest, status: "disabled" },
      supportsSaveUi: false
    };
    const deletedPack: CheatManagedPack = {
      ...activePack,
      manifest: { ...activePack.manifest, status: "deleted" },
      supportsSaveUi: false
    };
    vi.mocked(retrosaveApi.listCheatPacks)
      .mockResolvedValueOnce([activePack])
      .mockResolvedValueOnce([disabledPack])
      .mockResolvedValueOnce([activePack])
      .mockResolvedValueOnce([deletedPack]);

    render(
      <MemoryRouter>
        <CheatsPage />
      </MemoryRouter>
    );

    await screen.findByText("SM64 Runtime UI");

    fireEvent.click(screen.getByRole("button", { name: "Disable sm64-runtime-ui" }));
    await waitFor(() => {
      expect(retrosaveApi.disableCheatPack).toHaveBeenCalledWith("sm64-runtime-ui");
    });
    expect(await screen.findByText("disabled")).toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "Enable sm64-runtime-ui" }));
    await waitFor(() => {
      expect(retrosaveApi.enableCheatPack).toHaveBeenCalledWith("sm64-runtime-ui");
    });
    expect(await screen.findByText("active")).toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "Delete sm64-runtime-ui" }));
    await waitFor(() => {
      expect(retrosaveApi.deleteCheatPack).toHaveBeenCalledWith("sm64-runtime-ui");
    });
    expect(await screen.findByText("deleted")).toBeInTheDocument();
  });
});
