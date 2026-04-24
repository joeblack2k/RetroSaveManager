import { describe, expect, it } from "vitest";
import { buildSaveInsight } from "../saveInsights";
import type { SaveSummary } from "../../services/types";

function makeSave(overrides: Partial<SaveSummary>): SaveSummary {
  return {
    id: "save-1",
    game: {
      id: 1,
      name: "Test Game",
      displayTitle: "Test Game",
      regionCode: "US",
      regionFlag: "us",
      languageCodes: [],
      boxart: null,
      boxartThumb: null,
      hasParser: true,
      system: { id: 1, name: "Test System", slug: "test", manufacturer: "Test" }
    },
    filename: "test.sav",
    fileSize: 8192,
    format: "sram",
    version: 1,
    sha256: "sha",
    createdAt: "2026-04-24T12:00:00Z",
    metadata: {},
    ...overrides
  };
}

describe("buildSaveInsight", () => {
  it("renders parser-backed gameplay fields when a semantic decoder exposes them", () => {
    const insight = buildSaveInsight(makeSave({
      inspection: {
        parserLevel: "semantic",
        parserId: "example-game-decoder",
        trustLevel: "semantic-validated",
        checksumValid: true,
        semanticFields: {
          lives: 7,
          currentMap: "Green Hill",
          currentAct: 2
        }
      }
    }));

    expect(insight?.title).toBe("Gameplay decoder active");
    expect(insight?.rows).toContainEqual({ label: "Lives", value: "7", kind: "gameplay" });
    expect(insight?.rows).toContainEqual({ label: "Current map", value: "Green Hill", kind: "gameplay" });
    expect(insight?.rows).toContainEqual({ label: "Act", value: "2", kind: "gameplay" });
    expect(insight?.rows).toContainEqual({ label: "Checksum", value: "Valid", kind: "technical" });
  });

  it("explains structural-only saves without inventing lives or map data", () => {
    const insight = buildSaveInsight(makeSave({
      inspection: {
        parserLevel: "structural",
        parserId: "snes-dkc-family",
        validatedGameTitle: "Donkey Kong Country 3 - Dixie Kong's Double Trouble!",
        semanticFields: {
          variant: "dkc3",
          signatures: ["CRANK", "FUNK", "SWANK", "WRINKL"]
        }
      }
    }));

    expect(insight?.title).toBe("Save structure verified");
    expect(insight?.subtitle).toMatch(/exact lives or map need/i);
    expect(insight?.rows.some((row) => row.label === "Lives")).toBe(false);
    expect(insight?.rows).toContainEqual({
      label: "Validated game",
      value: "Donkey Kong Country 3 - Dixie Kong's Double Trouble!",
      kind: "technical"
    });
    expect(insight?.rows).toContainEqual({ label: "Game profile", value: "Donkey Kong Country 3", kind: "technical" });
  });

  it("shows raw media validation facts for generic saves", () => {
    const insight = buildSaveInsight(makeSave({
      inspection: {
        parserLevel: "container",
        parserId: "snes-raw-sram",
        payloadSizeBytes: 8192,
        semanticFields: {
          rawSaveKind: "SNES cartridge SRAM",
          blankCheck: "passed",
          extension: "srm",
          romLinked: true,
          mediaType: "sram",
          nonZeroBytes: 528,
          nonFFBytes: 7680
        }
      }
    }));

    expect(insight?.title).toBe("Raw cartridge save verified");
    expect(insight?.subtitle).toMatch(/NES, Game Boy, GBA, SNES, and Sega raw saves/i);
    expect(insight?.rows).toContainEqual({ label: "Raw save kind", value: "SNES cartridge SRAM", kind: "technical" });
    expect(insight?.rows).toContainEqual({ label: "ROM link", value: "Present", kind: "technical" });
    expect(insight?.rows).toContainEqual({ label: "Blank check", value: "passed", kind: "technical" });
    expect(insight?.rows).toContainEqual({ label: "File extension", value: "srm", kind: "technical" });
    expect(insight?.rows).toContainEqual({ label: "Payload size", value: "8.0 KB", kind: "technical" });
    expect(insight?.rows).toContainEqual({ label: "Non-zero bytes", value: "528", kind: "technical" });
  });
});
