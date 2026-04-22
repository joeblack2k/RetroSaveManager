import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MyGamesPage } from "../MyGamesPage";
import * as retrosaveApi from "../../../services/retrosaveApi";
import type { SaveSummary } from "../../../services/types";

vi.mock("../../../services/retrosaveApi", () => ({
  listSaves: vi.fn(),
  deleteManySaves: vi.fn(),
  deleteSave: vi.fn()
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
    systemSlug,
    regionCode: overrides.regionCode ?? "US",
    regionFlag: "us",
    languageCodes: [],
    coverArtUrl: undefined,
    saveCount: overrides.saveCount ?? 1,
    latestSizeBytes: overrides.latestSizeBytes ?? 4096,
    totalSizeBytes: overrides.totalSizeBytes ?? 8192,
    latestVersion: overrides.latestVersion ?? 2,
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
        latestVersion: 4
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
      })
    ]);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders English treegrid copy and supports expand/collapse", async () => {
    renderPage();

    expect(screen.getByRole("status", { name: "" })).toBeInTheDocument();
    expect(await screen.findByRole("treegrid", { name: "My Saves" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "My Saves" })).toBeInTheDocument();
    expect(screen.getByText(/2 systems · 3 games · 3 saves/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /collapse playstation/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /expand super nintendo/i })).toBeInTheDocument();
    expect(screen.queryByText("Chrono Trigger")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /expand super nintendo/i }));

    expect(await screen.findByText("Chrono Trigger")).toBeInTheDocument();
  });

  it("sorts rows inside a console group and keeps PlayStation links scoped with psLogicalKey", async () => {
    const view = renderPage();

    await screen.findByRole("treegrid", { name: "My Saves" });
    await waitFor(() => {
      expect(titlesForGroup(view.container, "psx")).toEqual(["Resident Evil 2", "Ape Escape"]);
    });

    fireEvent.click(screen.getByRole("button", { name: /sort by gamename/i }));

    await waitFor(() => {
      expect(titlesForGroup(view.container, "psx")).toEqual(["Ape Escape", "Resident Evil 2"]);
    });

    const detailsLink = screen.getByRole("link", { name: /view details for resident evil 2/i });
    const downloadLink = screen.getByRole("link", { name: /download resident evil 2/i });

    expect(detailsLink).toHaveAttribute(
      "href",
      "/app/saves/ps-save-2?psLogicalKey=psx%3A%3ASLUS-00748%3A%3Aresident%20evil%202%3A%3AUS"
    );
    expect(downloadLink).toHaveAttribute(
      "href",
      "/saves/download?id=ps-save-2&psLogicalKey=psx%3A%3ASLUS-00748%3A%3Aresident%20evil%202%3A%3AUS"
    );
  });
});
