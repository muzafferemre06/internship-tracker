import { describe, expect, it } from "vitest";
import { coverageForPriority, coverageStatusLabels, coverageTone, formatCoveragePercent, programStatusLabels, type CoverageResponse } from "./coverage";

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

  it("keeps primary and secondary coverage summaries separate", () => {
    const coverage = {
      priority_summaries: {
        primary: { total_companies: 12, total_sources: 14, automatic_sources: 6, feed_sources: 0, manual_sources: 4, researching_sources: 4, broken_sources: 0, automatic_eligible_sources: 10, automatic_coverage_percent: 60 },
        secondary: { total_companies: 15, total_sources: 15, automatic_sources: 4, feed_sources: 0, manual_sources: 5, researching_sources: 6, broken_sources: 0, automatic_eligible_sources: 10, automatic_coverage_percent: 40 },
      },
    } as CoverageResponse;

    expect(coverageForPriority(coverage, "primary").total_companies).toBe(12);
    expect(coverageForPriority(coverage, "secondary").automatic_coverage_percent).toBe(40);
  });
});
