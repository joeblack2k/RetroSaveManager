import { describe, expect, it } from "vitest";
import { buildSaveDetailsHref, buildSaveDownloadHref, buildSaveRows } from "../saveRows";
import type { SaveSummary } from "../../services/types";

function makeSave(overrides: Partial<SaveSummary>): SaveSummary {
  return {
    id: "save-1",
    game: {
      id: 1,
      name: "Burnout 3",
      displayTitle: "Burnout 3",
      regionCode: "US",
      regionFlag: "us",
      languageCodes: [],
      coverArtUrl: undefined,
      boxart: null,
      boxartThumb: null,
      hasParser: true,
      system: { id: 1, name: "PlayStation 2", slug: "ps2", manufacturer: "Sony" }
    },
    displayTitle: "Burnout 3",
    systemSlug: "ps2",
    regionCode: "US",
    regionFlag: "us",
    languageCodes: [],
    coverArtUrl: undefined,
    saveCount: 1,
    latestSizeBytes: 81920,
    totalSizeBytes: 81920,
    latestVersion: 1,
    memoryCard: null,
    filename: "Burnout 3.zip",
    fileSize: 81920,
    format: "zip",
    version: 1,
    sha256: "sha",
    createdAt: "2026-04-21T15:11:47Z",
    metadata: {},
    ...overrides
  };
}

describe("saveRows", () => {
  it("keeps PlayStation logical detail links scoped to the logical save", () => {
    expect(buildSaveDetailsHref({ primarySaveID: "save-42", psLogicalKey: "ps2::BASLUS-21050::burnout 3::US" })).toBe(
      "/app/saves/save-42?psLogicalKey=ps2%3A%3ABASLUS-21050%3A%3Aburnout%203%3A%3AUS"
    );
  });

  it("filters out raw PlayStation card blobs from GUI rows", () => {
    const rows = buildSaveRows([
      makeSave({ id: "projection-save", logicalKey: "ps2::BASLUS-21050::burnout 3::US" }),
      makeSave({ id: "projection-save", displayTitle: "Memory Card 1", logicalKey: undefined, filename: "memory_card_1.ps2" })
    ]);

    expect(rows).toHaveLength(1);
    expect(rows[0]?.gameName).toBe("Burnout 3");
    expect(rows[0]?.psLogicalKey).toBe("ps2::BASLUS-21050::burnout 3::US");
  });

  it("builds profile-aware download URLs while preserving logical keys", () => {
    expect(buildSaveDownloadHref({
      saveId: "save-42",
      psLogicalKey: "ps2::BASLUS-21050::burnout 3::US",
      revisionId: "rev-7"
    }, "ps2/pcsx2")).toBe(
      "/saves/download?id=save-42&psLogicalKey=ps2%3A%3ABASLUS-21050%3A%3Aburnout+3%3A%3AUS&revisionId=rev-7&runtimeProfile=ps2%2Fpcsx2"
    );
  });
});
