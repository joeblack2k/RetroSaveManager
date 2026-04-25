import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MyGamesPage } from "../MyGamesPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { SaveSummary } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  listSaves: vi.fn(),
  deleteManySaves: vi.fn(),
  deleteSave: vi.fn(),
  getSaveHistory: vi.fn(),
  rollbackSave: vi.fn(),
  getSaveCheats: vi.fn(),
  applySaveCheats: vi.fn(),
  uploadSaveFile: vi.fn()
}));

function makeSave(overrides: Partial<SaveSummary> & { id: string; title: string; systemSlug: string; systemName: string }): SaveSummary {
  const { id, title, systemSlug, systemName, ...rest } = overrides;
  return {
    id,
    game: {
      id: Number(id.replace(/\D+/g, "")) || 1,
      name: title,
      displayTitle: title,
      regionCode: overrides.regionCode ?? "US",
      regionFlag: "us",
      languageCodes: [],
      coverArtUrl: undefined,
      boxart: null,
      boxartThumb: null,
      hasParser: true,
      system: { id: 1, name: systemName, slug: systemSlug, manufacturer: "Test" }
    },
    displayTitle: title,
    logicalKey: overrides.logicalKey,
    downloadProfiles: overrides.downloadProfiles ?? [{ id: "original", label: "Original file", targetExtension: ".sav" }],
    systemSlug,
    regionCode: overrides.regionCode ?? "US",
    regionFlag: "us",
    languageCodes: [],
    coverArtUrl: undefined,
    saveCount: overrides.saveCount ?? 1,
    latestSizeBytes: overrides.latestSizeBytes ?? 4096,
    totalSizeBytes: overrides.totalSizeBytes ?? 8192,
    latestVersion: overrides.latestVersion ?? 2,
    cheats: overrides.cheats ?? null,
    memoryCard: null,
    filename: overrides.filename ?? `${title}.zip`,
    fileSize: overrides.fileSize ?? 4096,
    format: overrides.format ?? "zip",
    version: overrides.version ?? 2,
    sha256: overrides.sha256 ?? `${id}-sha`,
    createdAt: overrides.createdAt ?? "2026-04-22T08:00:00Z",
    metadata: overrides.metadata ?? {},
    ...rest
  };
}

function renderPage(): ReturnType<typeof render> {
  return render(
    <MemoryRouter>
      <MyGamesPage />
    </MemoryRouter>
  );
}

function titlesForGroup(container: HTMLElement, groupKey: string): string[] {
  return Array.from(container.querySelectorAll(`tr[data-treegrid-group="${groupKey}"] .treegrid-game-title`))
    .map((element) => element.textContent?.trim() || "")
    .filter((value) => value !== "");
}

describe("MyGamesPage TreeGrid", () => {
  beforeEach(() => {
    vi.mocked(retrosaveApi.listSaves).mockResolvedValue([
      makeSave({
        id: "ps-save-2",
        title: "Resident Evil 2",
        systemSlug: "psx",
        systemName: "PlayStation",
        logicalKey: "psx::SLUS-00748::resident evil 2::US",
        createdAt: "2026-04-22T10:00:00Z",
        latestVersion: 4,
        saveCount: 3
      }),
      makeSave({
        id: "ps-save-1",
        title: "Ape Escape",
        systemSlug: "psx",
        systemName: "PlayStation",
        logicalKey: "psx::SCUS-94423::ape escape::US",
        createdAt: "2026-04-21T10:00:00Z",
        latestVersion: 3
      }),
      makeSave({
        id: "snes-save-1",
        title: "Chrono Trigger",
        systemSlug: "snes",
        systemName: "Super Nintendo",
        createdAt: "2026-04-20T10:00:00Z"
      }),
      makeSave({
        id: "sm64-save-1",
        title: "Super Mario 64",
        systemSlug: "n64",
        systemName: "Nintendo 64",
        createdAt: "2026-04-19T08:00:00Z",
        cheats: { supported: true, availableCount: 4, editorId: "sm64-eeprom" }
      })
    ]);
    vi.mocked(retrosaveApi.getSaveHistory).mockResolvedValue({
      success: true,
      game: null,
      displayTitle: "Resident Evil 2",
      systemSlug: "psx",
      versions: [
        makeSave({
          id: "ps-save-2",
          title: "Resident Evil 2",
          systemSlug: "psx",
          systemName: "PlayStation",
          logicalKey: "psx::SLUS-00748::resident evil 2::US",
          createdAt: "2026-04-22T10:00:00Z",
          version: 4
        }),
        makeSave({
          id: "ps-save-0",
          title: "Resident Evil 2",
          systemSlug: "psx",
          systemName: "PlayStation",
          logicalKey: "psx::SLUS-00748::resident evil 2::US",
          createdAt: "2026-04-20T08:00:00Z",
          version: 3
        })
      ]
    });
    vi.mocked(retrosaveApi.rollbackSave).mockResolvedValue({
      success: true,
      sourceSaveId: "ps-save-0",
      save: makeSave({
        id: "ps-save-3",
        title: "Resident Evil 2",
        systemSlug: "psx",
        systemName: "PlayStation",
        logicalKey: "psx::SLUS-00748::resident evil 2::US",
        createdAt: "2026-04-22T10:10:00Z",
        version: 5
      })
    });
    vi.mocked(retrosaveApi.getSaveCheats).mockResolvedValue({
      success: true,
      saveId: "sm64-save-1",
      displayTitle: "Super Mario 64",
      cheats: {
        supported: true,
        editorId: "sm64-eeprom",
        availableCount: 4,
        selector: {
          id: "file",
          label: "Save File",
          type: "save-file",
          options: [
            { id: "A", label: "File A" },
            { id: "B", label: "File B" }
          ]
        },
        sections: [
          {
            id: "abilities",
            title: "Abilities",
            fields: [{ id: "haveWingCap", label: "Wing Cap Switch", type: "boolean" }]
          }
        ],
        presets: [{ id: "unlockCaps", label: "Unlock All Caps", updates: { haveWingCap: true } }],
        slotValues: {
          A: { haveWingCap: false },
          B: { haveWingCap: true }
        }
      }
    });
    vi.mocked(retrosaveApi.applySaveCheats).mockResolvedValue({
      success: true,
      sourceSaveId: "sm64-save-1",
      save: makeSave({
        id: "sm64-save-2",
        title: "Super Mario 64",
        systemSlug: "n64",
        systemName: "Nintendo 64",
        version: 2,
        cheats: { supported: true, availableCount: 4, editorId: "sm64-eeprom" }
      })
    });
    vi.mocked(retrosaveApi.uploadSaveFile).mockResolvedValue({
      success: true,
      save: { id: "wii-save-1", sha256: "wii-sha", version: 1 },
      successCount: 1,
      errorCount: 0
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders English treegrid copy and supports expand/collapse", async () => {
    renderPage();

    expect(screen.getByRole("status", { name: "" })).toBeInTheDocument();
    expect(await screen.findByRole("treegrid", { name: "My Saves" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "My Saves" })).toBeInTheDocument();
    expect(screen.getByText(/3 systems · 4 games · 6 saves/i)).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /collapse nintendo 64/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /expand playstation/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /expand super nintendo/i })).toBeInTheDocument();
    expect(screen.queryByText("Chrono Trigger")).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: /rollback/i })).not.toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: /cheats/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /edit cheats for super mario 64/i })).toHaveTextContent("4 cheats");

    fireEvent.click(screen.getByRole("button", { name: /expand super nintendo/i }));

    expect(await screen.findByText("Chrono Trigger")).toBeInTheDocument();
  });

  it("sorts rows inside a console group and opens the download profile popup with the preserved PlayStation key", async () => {
    const view = renderPage();

    await screen.findByRole("treegrid", { name: "My Saves" });
    fireEvent.click(screen.getByRole("button", { name: /expand playstation/i }));
    await waitFor(() => {
      expect(titlesForGroup(view.container, "psx")).toEqual(["Resident Evil 2", "Ape Escape"]);
    });

    fireEvent.click(screen.getByRole("button", { name: /sort by gamename/i }));

    await waitFor(() => {
      expect(titlesForGroup(view.container, "psx")).toEqual(["Ape Escape", "Resident Evil 2"]);
    });

    const detailsLink = screen.getByRole("link", { name: /view details for resident evil 2/i });
    const downloadButton = screen.getByRole("button", { name: /download resident evil 2/i });

    expect(detailsLink).toHaveAttribute(
      "href",
      "/app/saves/ps-save-2?psLogicalKey=psx%3A%3ASLUS-00748%3A%3Aresident%20evil%202%3A%3AUS"
    );

    fireEvent.click(downloadButton);

    expect(await screen.findByRole("heading", { name: "Download Save" })).toBeInTheDocument();
    const modalDownloadLink = screen.getByRole("link", { name: "Download" });
    expect(modalDownloadLink).toHaveAttribute(
      "href",
      "/saves/download?id=ps-save-2&psLogicalKey=psx%3A%3ASLUS-00748%3A%3Aresident+evil+2%3A%3AUS"
    );
  });

  it("opens the sync save selector modal from the saves column and promotes the chosen version", async () => {
    renderPage();

    await screen.findByRole("treegrid", { name: "My Saves" });
    fireEvent.click(screen.getByRole("button", { name: /expand playstation/i }));

    fireEvent.click(screen.getByRole("button", { name: /select sync save for resident evil 2/i }));

    expect(await screen.findByRole("heading", { name: "Select Sync Save" })).toBeInTheDocument();
    expect(retrosaveApi.getSaveHistory).toHaveBeenCalledWith({
      saveId: "ps-save-2",
      psLogicalKey: "psx::SLUS-00748::resident evil 2::US"
    });
    expect(screen.getByText("Current Sync Save")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /select version 3 for sync/i }));

    await waitFor(() => {
      expect(retrosaveApi.rollbackSave).toHaveBeenCalledWith({
        saveId: "ps-save-2",
        psLogicalKey: "psx::SLUS-00748::resident evil 2::US",
        revisionId: "ps-save-0"
      });
    });
  });

  it("opens the cheat editor modal and applies structured cheat changes", async () => {
    renderPage();

    await screen.findByRole("treegrid", { name: "My Saves" });
    fireEvent.click(screen.getByRole("button", { name: /edit cheats for super mario 64/i }));

    expect(await screen.findByRole("heading", { name: "Cheat Editor" })).toBeInTheDocument();
    expect(retrosaveApi.getSaveCheats).toHaveBeenCalledWith("sm64-save-1", undefined);

    fireEvent.click(screen.getByRole("button", { name: /unlock all caps/i }));
    fireEvent.click(screen.getByRole("button", { name: /apply cheats/i }));

    await waitFor(() => {
      expect(retrosaveApi.applySaveCheats).toHaveBeenCalledWith({
        saveId: "sm64-save-1",
        psLogicalKey: undefined,
        editorId: "sm64-eeprom",
        slotId: "A",
        updates: { haveWingCap: true }
      });
    });
  });

  it("uploads a save file from the My Saves header and refreshes the list", async () => {
    renderPage();

    await screen.findByRole("treegrid", { name: "My Saves" });
    fireEvent.click(screen.getByRole("button", { name: "Upload" }));

    expect(await screen.findByRole("heading", { name: "Upload Save" })).toBeInTheDocument();
    const file = new File([new Uint8Array([1, 2, 3, 4])], "data.bin", { type: "application/octet-stream" });
    fireEvent.change(screen.getByLabelText(/save file or zip/i), { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText(/^system$/i), { target: { value: "wii" } });
    fireEvent.change(screen.getByLabelText(/wii title code/i), { target: { value: "SB4P" } });

    fireEvent.click(screen.getByRole("button", { name: /import save/i }));

    await waitFor(() => {
      expect(retrosaveApi.uploadSaveFile).toHaveBeenCalledWith({
        file,
        system: "wii",
        slotName: undefined,
        romSha1: undefined,
        wiiTitleId: "SB4P"
      });
    });
    expect(await screen.findByText("1 save imported.")).toBeInTheDocument();
    expect(retrosaveApi.listSaves).toHaveBeenCalledTimes(2);
  });
});
