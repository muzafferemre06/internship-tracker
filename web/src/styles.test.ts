import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const styles = readFileSync(new URL("./styles.css", import.meta.url), "utf8");

function declarationBlock(selector: string): string {
  const start = styles.indexOf(`${selector} {`);
  if (start < 0) return "";
  const bodyStart = styles.indexOf("{", start) + 1;
  const bodyEnd = styles.indexOf("}", bodyStart);
  return styles.slice(bodyStart, bodyEnd);
}

describe("long error text layout", () => {
  it("allows coverage source errors to shrink and wrap inside their cards", () => {
    expect(declarationBlock(".coverage-list li")).toMatch(/min-width:\s*0/);
    expect(declarationBlock(".coverage-sources a")).toMatch(/min-width:\s*0/);
    expect(declarationBlock(".coverage-sources small")).toMatch(/white-space:\s*normal/);
    expect(declarationBlock(".coverage-sources small")).toMatch(/overflow-wrap:\s*anywhere/);
  });

  it("contains long dashboard and manual-source errors", () => {
    expect(declarationBlock(".status")).toMatch(/overflow-wrap:\s*anywhere/);
    expect(declarationBlock(".manual-list li > div")).toMatch(/min-width:\s*0/);
  });
});
