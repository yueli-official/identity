import { expect, test, vi } from "vitest";

test("pre-bundles login validation before the first client hydration", async () => {
  vi.stubGlobal("defineNuxtConfig", <Config>(config: Config) => config);
  const config = (await import("../nuxt.config")).default as {
    vite?: { optimizeDeps?: { include?: string[] } };
    devtools?: { enabled?: boolean };
    experimental?: { appManifest?: boolean };
  };

  expect(config.vite?.optimizeDeps?.include).toContain("zod");
  expect(config.devtools?.enabled).toBe(false);
  expect(config.experimental?.appManifest).toBe(false);
});
