import { describe, expect, it } from "vitest";
import { companiesForSection, coverageForPriority, coverageForSection, coverageReasonLabels, coverageStatusLabels, coverageTone, formatCoveragePercent, programStatusLabels, type CoverageResponse } from "./coverage";

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

  it("presents Phase 16.5 as a separate section without changing business priority", () => {
	const empty = { total_companies: 0, total_sources: 0, automatic_sources: 0, feed_sources: 0, manual_sources: 0, researching_sources: 0, broken_sources: 0, automatic_eligible_sources: 0, automatic_coverage_percent: 0 };
	const coverage = {
	  section_summaries: { primary: empty, secondary: empty, phase_16_5: { ...empty, total_companies: 1, total_sources: 1, manual_sources: 1 } },
	  companies: [
		{ name: "İnnova", priority: "secondary", tracking_phase: "16.5", tracking_status: "active", sources: [] },
		{ name: "Evreka", priority: "secondary", tracking_status: "active", sources: [] },
	  ],
	} as unknown as CoverageResponse;

	expect(coverageForSection(coverage, "phase_16_5").total_companies).toBe(1);
	expect(companiesForSection(coverage, "phase_16_5").map((company) => company.name)).toEqual(["İnnova"]);
	expect(companiesForSection(coverage, "secondary").map((company) => company.name)).toEqual(["Evreka"]);
	expect(coverageReasonLabels.account_required).toBe("Aday hesabı gerekiyor");
  });
});
