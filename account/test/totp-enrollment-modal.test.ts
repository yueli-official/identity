import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("TOTP enrollment modal", () => {
  it("requires an explicit close action throughout enrollment", () => {
    const source = readFileSync(
      new URL("../app/components/TOTPManager.vue", import.meta.url),
      "utf8",
    );
    const enrollmentModal = source.match(
      /<UModal[\s\S]*?title="设置身份验证器"[\s\S]*?>/,
    )?.[0];

    expect(enrollmentModal).toBeDefined();
    expect(enrollmentModal).toContain(':dismissible="false"');
  });
});
