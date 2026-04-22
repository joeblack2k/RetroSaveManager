import { describe, expect, it } from "vitest";
import { normalizeConsoleLabel } from "../../../utils/saveRows";

describe("MyGamesPage console label normalization", () => {
  it("keeps SNES separate from NES when the slug is snes", () => {
    expect(normalizeConsoleLabel("snes", "Nintendo Super Nintendo Entertainment System")).toEqual({
      slug: "snes",
      name: "Super Nintendo"
    });
  });

  it("maps NES labels correctly", () => {
    expect(normalizeConsoleLabel("nes", "Nintendo Entertainment System")).toEqual({
      slug: "nes",
      name: "Nintendo Entertainment System"
    });
  });
});
