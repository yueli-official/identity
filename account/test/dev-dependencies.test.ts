import { expect, test, vi } from "vitest";

test("pre-bundles login validation before the first client hydration", async () => {
  vi.stubGlobal("defineNuxtConfig", <Config>(config: Config) => config);
  const config = (await import("../nuxt.config")).default as {
    vite?: { optimizeDeps?: { include?: string[] } };
  };

  expect(config.vite?.optimizeDeps?.include).toContain("zod");
});
