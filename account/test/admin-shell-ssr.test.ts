import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const layout = readFileSync(
  fileURLToPath(new URL("../app/layouts/admin.vue", import.meta.url)),
  "utf8",
);

describe("Identity 管理壳 SSR", () => {
  it("不使用整壳 ClientOnly 或固定控制台中转页", () => {
    expect(layout).toMatch(/<template>\s*<YAdminConsoleLayout/);
    expect(layout).not.toMatch(/<template>\s*<ClientOnly>[\s\S]*?<YAdminConsoleLayout/);
    expect(layout).not.toMatch(/正在打开[^\n]{0,16}控制台/);
    expect(layout).not.toContain("secondaryNavigation");
  });
});
