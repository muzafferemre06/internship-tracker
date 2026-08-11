import { describe, expect, it } from "vitest";
import { coverageStatusLabels, coverageTone, formatCoveragePercent, programStatusLabels } from "./coverage";

describe("coverage presentation", () => {
  it("labels every backend coverage state without merging manual and researching", () => {
    expect(coverageStatusLabels).toEqual({
      automatic: "Otomatik",
      feed: "Akış",
      manual: "Manuel",
      researching: "Araştırılıyor",
      broken: "Bozuk",
    });
    expect(coverageTone("manual")).not.toBe(coverageTone("researching"));
    expect(coverageTone("broken")).toBe("danger");
  });

  it("formats the automatic ratio and program state", () => {
    expect(formatCoveragePercent(62.5)).toBe("%62,5");
    expect(programStatusLabels.closed).toBe("Başvuru kapalı");
  });
});
