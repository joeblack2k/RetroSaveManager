import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SaveDetailPage } from "../SaveDetailPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { SaveHistoryResponse, SaveSummary } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  getSaveHistory: vi.fn(),
  getSaveCheats: vi.fn(),
  rollbackSave: vi.fn()
}));

function makeSave(overrides: Partial<SaveSummary> = {}): SaveSummary {
  const title = overrides.displayTitle || overrides.game?.displayTitle || "Sonic the Hedgehog 3";
  return {
    id: overrides.id || "save-1",
    game: {
      id: 1,
      name: title,
      displayTitle: title,
      regionCode: "US",
      regionFlag: "us",
      languageCodes: ["en"],
      boxart: null,
      boxartThumb: null,
      hasParser: true,
      system: { id: 1, name: "Sega Genesis", slug: "genesis", manufacturer: "Sega" },
      ...overrides.game
    },
    displayTitle: title,
    systemSlug: overrides.systemSlug || "genesis",
    regionCode: overrides.regionCode || "US",
    regionFlag: "us",
    languageCodes: overrides.languageCodes || ["en"],
    downloadProfiles: overrides.downloadProfiles || [{ id: "original", label: "Original file", targetExtension: ".srm" }],
    filename: overrides.filename || "sonic3.srm",
    fileSize: overrides.fileSize || 8192,
    format: overrides.format || "srm",
    version: overrides.version || 2,
    sha256: overrides.sha256 || "abc123sha",
    createdAt: overrides.createdAt || "2026-04-25T10:00:00Z",
    metadata: {},
    ...overrides
  };
}

function renderDetail(response: SaveHistoryResponse): ReturnType<typeof render> {
  vi.mocked(retrosaveApi.getSaveHistory).mockResolvedValue(response);
  return render(
    <MemoryRouter initialEntries={["/app/saves/save-1"]}>
      <Routes>
        <Route path="/app/saves/:saveId" element={<SaveDetailPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe("SaveDetailPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
  });

  it("renders a gameplay-first detail page when parser semantic fields are available", async () => {
    renderDetail({
      success: true,
      game: null,
      displayTitle: "Sonic the Hedgehog 3",
      systemSlug: "genesis",
      summary: {
        displayTitle: "Sonic the Hedgehog 3",
        system: { id: 1, name: "Sega Genesis", slug: "genesis" },
        regionCode: "US",
        regionFlag: "us",
        languageCodes: ["en"],
        saveCount: 2,
        totalSizeBytes: 16384,
        latestVersion: 2,
        latestCreatedAt: "2026-04-25T10:00:00Z"
      },
      versions: [
        makeSave({
          inspection: {
            parserLevel: "semantic",
            parserId: "sonic3-save",
            validatedGameTitle: "Sonic the Hedgehog 3",
            checksumValid: true,
            semanticFields: {
              lives: 7,
              currentZone: "Angel Island",
              currentAct: 2,
              emeraldCount: 5,
              nonZeroBytes: 128,
              extension: "srm"
            }
          }
        }),
        makeSave({ id: "save-0", version: 1, createdAt: "2026-04-24T10:00:00Z" })
      ]
    });

    expect(await screen.findByRole("heading", { name: "Sonic the Hedgehog 3" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Gameplay decoder active" })).toBeInTheDocument();
    expect(screen.getByText("Lives")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Zone")).toBeInTheDocument();
    expect(screen.getByText("Angel Island")).toBeInTheDocument();
    expect(screen.getByText("Emerald Count")).toBeInTheDocument();
    expect(screen.getByText("Current sync")).toBeInTheDocument();
    expect(screen.getByText("Verified technical data")).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "SHA256" })).not.toBeInTheDocument();
  });

  it("shows cheat-backed values when a safe editor can read the save", async () => {
    vi.mocked(retrosaveApi.getSaveCheats).mockResolvedValue({
      success: true,
      saveId: "save-1",
      displayTitle: "Metal Slug 5",
      cheats: {
        supported: true,
        gameId: "neogeo/mslug5",
        systemSlug: "neogeo",
        editorId: "neogeo-mslug5",
        adapterId: "neogeo-mslug5",
        packId: "neogeo--mslug5",
        title: "Metal Slug 5",
        availableCount: 1,
        values: { freePlay: false },
        sections: [
          {
            id: "cabinet",
            title: "Cabinet",
            fields: [
              {
                id: "freePlay",
                ref: "freePlay",
                label: "Free Play",
                type: "boolean"
              }
            ]
          }
        ],
        presets: []
      }
    });

    renderDetail({
      success: true,
      game: null,
      displayTitle: "Metal Slug 5",
      systemSlug: "neogeo",
      versions: [
        makeSave({
          displayTitle: "Metal Slug 5",
          systemSlug: "neogeo",
          cheats: { supported: true, availableCount: 1, editorId: "neogeo-mslug5" },
          game: {
            id: 3,
            name: "Metal Slug 5",
            displayTitle: "Metal Slug 5",
            regionCode: "UNKNOWN",
            regionFlag: "unknown",
            languageCodes: [],
            boxart: null,
            boxartThumb: null,
            hasParser: true,
            system: { id: 3, name: "Neo Geo", slug: "neogeo", manufacturer: "SNK" }
          },
          inspection: {
            parserLevel: "container",
            parserId: "neogeo-raw-save",
            semanticFields: {
              layout: "compound",
              nonPaddingBytes: 2184
            }
          }
        })
      ]
    });

    expect(await screen.findByRole("heading", { name: "Metal Slug 5" })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Cheat-backed save values" })).toBeInTheDocument();
    expect(screen.getByText("Free Play")).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "No gameplay facts yet" })).not.toBeInTheDocument();
  });

  it("keeps raw media details calm when no gameplay parser exists yet", async () => {
    renderDetail({
      success: true,
      game: null,
      displayTitle: "Wario Land 4",
      systemSlug: "gba",
      versions: [
        makeSave({
          displayTitle: "Wario Land 4",
          systemSlug: "gba",
          game: {
            id: 2,
            name: "Wario Land 4",
            displayTitle: "Wario Land 4",
            regionCode: "US",
            regionFlag: "us",
            languageCodes: ["en"],
            boxart: null,
            boxartThumb: null,
            hasParser: false,
            system: { id: 2, name: "Game Boy Advance", slug: "gba", manufacturer: "Nintendo" }
          },
          inspection: {
            parserLevel: "container",
            parserId: "gba-raw-backup",
            semanticFields: {
              rawSaveKind: "Game Boy Advance backup memory",
              extension: "srm",
              nonZeroBytes: 249,
              romLinked: true
            }
          }
        })
      ]
    });

    expect(await screen.findByRole("heading", { name: "Wario Land 4" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Raw cartridge save verified" })).toBeInTheDocument();
    expect(screen.getAllByText("Raw save kind").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Game Boy Advance backup memory").length).toBeGreaterThan(0);
    expect(screen.queryByRole("heading", { name: "No gameplay facts yet" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "SHA256" })).not.toBeInTheDocument();
  });
});
