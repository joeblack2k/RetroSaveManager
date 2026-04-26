import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ValidationPage } from "../ValidationPage";
import { deleteQuarantineItem, getValidationStatus, rescanValidation, retryQuarantineItem } from "../../../services/retrosaveApi";

vi.mock("../../../services/retrosaveApi", () => ({
  getValidationStatus: vi.fn(() =>
    Promise.resolve({
      generatedAt: "2026-04-26T10:00:00Z",
      counts: { mediaVerified: 2, romVerified: 1, structureVerified: 3, semanticVerified: 4, unknown: 1 },
      systems: { snes: 3, n64: 2 },
      quarantineCount: 1,
      coverageSummary: { total: 2, gameplayFacts: 1, semantic: 1, cheats: 1, missing: 1 },
      coverage: [
        {
          saveId: "save-1",
          displayTitle: "Sonic the Hedgehog 3",
          systemSlug: "genesis",
          systemName: "Sega Genesis",
          parserLevel: "semantic",
          parserId: "sonic3-save",
          gameplayFactCount: 4,
          hasGameplayFacts: true,
          cheatsSupported: true,
          cheatCount: 1,
          updatedAt: "2026-04-26T09:00:00Z"
        }
      ],
      quarantine: [
        {
          id: "q1",
          filename: "notes.txt",
          payloadFile: "payload.txt",
          sizeBytes: 32,
          sha256: "sha",
          reason: "text payload",
          systemSlug: "unknown-system",
          format: "txt",
          parserLevel: "none",
          trustLevel: "none",
          uploadedAt: "2026-04-26T09:30:00Z",
          uploadSource: "test-helper"
        }
      ]
    })
  ),
  rescanValidation: vi.fn(() =>
    Promise.resolve({
      success: true,
      result: { scanned: 10, updated: 2, rejected: 1, duplicateVersionsRemoved: 3 },
      validation: {
        generatedAt: "2026-04-26T10:01:00Z",
        counts: { mediaVerified: 2, romVerified: 1, structureVerified: 3, semanticVerified: 4, unknown: 1 },
        systems: { snes: 3, n64: 2 },
        quarantineCount: 1,
        coverageSummary: { total: 2, gameplayFacts: 1, semantic: 1, cheats: 1, missing: 1 },
        coverage: [],
        quarantine: []
      }
    })
  ),
  retryQuarantineItem: vi.fn(() => Promise.resolve({ success: true, imported: true, message: "Quarantined file imported." })),
  deleteQuarantineItem: vi.fn(() => Promise.resolve({ success: true, deleted: "q1" }))
}));

describe("ValidationPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders validation counts and can trigger a rescan", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    render(<ValidationPage />);

    expect(await screen.findByText("Validation")).toBeInTheDocument();
    expect(screen.getByText("Semantic")).toBeInTheDocument();
    expect(screen.getByText("Gameplay Coverage")).toBeInTheDocument();
    expect(screen.getByText("Sonic the Hedgehog 3")).toBeInTheDocument();
    expect(screen.getAllByText("notes.txt").length).toBeGreaterThan(0);
    expect(screen.getByText("text payload")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Rescan and repair" }));

    await waitFor(() => {
      expect(rescanValidation).toHaveBeenCalledWith({ dryRun: false, pruneUnsupported: true });
    });
    expect(await screen.findByText(/10 scanned, 2 updated, 1 rejected, 3 duplicate versions removed/i)).toBeInTheDocument();
    expect(getValidationStatus).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => {
      expect(retryQuarantineItem).toHaveBeenCalledWith("q1");
    });

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => {
      expect(deleteQuarantineItem).toHaveBeenCalledWith("q1");
    });
  });
});
