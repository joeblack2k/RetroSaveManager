import { describe, expect, it } from "vitest";
import { formatBytes, formatDate } from "../format";

describe("format helpers", () => {
  it("formats bytes with units", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
  });

  it("formats dates or falls back", () => {
    expect(formatDate("2026-04-21T12:00:00Z")).toContain("2026");
    expect(formatDate("not-a-date")).toBe("not-a-date");
  });
});
