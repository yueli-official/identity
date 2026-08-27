import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const plugin = readFileSync(
  path.resolve(import.meta.dirname, "../app/plugins/auth.ts"),
  "utf8",
);

describe("auth hydration reconciliation", () => {
  it("reconciles the browser cookie even when SSR already populated the user", () => {
    expect(plugin).toContain("if (import.meta.client)");
    expect(plugin).toMatch(/if \(import\.meta\.client\) \{[\s\S]*?await refresh\(\)/);
    expect(plugin).toMatch(/if \(!user\.value\) await refresh\(\)/);
  });
});
