import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const projectFile = (path: string) =>
  fileURLToPath(new URL(`../${path}`, import.meta.url));

describe("public profile routes", () => {
  const handlePage = projectFile("app/pages/@[handle].vue");
  const stablePage = projectFile("app/pages/u/[userKey].vue");
  const profile = projectFile("app/components/PublicUserProfile.vue");

  it("ships both the preferred handle route and the immutable user-key route", () => {
    expect(existsSync(handlePage)).toBe(true);
    expect(existsSync(stablePage)).toBe(true);
    expect(existsSync(profile)).toBe(true);
  });

  it("resolves each route through the public Identity API", () => {
    if (!existsSync(handlePage) || !existsSync(stablePage)) return;

    expect(readFileSync(handlePage, "utf8")).toContain(
      "/api/v1/users/by-handle/",
    );
    expect(readFileSync(stablePage, "utf8")).toContain("/api/v1/users/");
  });

  it("keeps private account fields out of the public presentation", () => {
    if (!existsSync(profile)) return;

    const source = readFileSync(profile, "utf8");
    expect(source).toContain("user.userKey");
    expect(source).toContain("user.displayName");
    expect(source).not.toMatch(/user\.(?:email|roles|status|id)\b/);
  });
});
