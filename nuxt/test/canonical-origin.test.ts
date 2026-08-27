import { describe, expect, it } from "vitest";

import { canonicalPageURL } from "../server/utils/canonical-origin";

describe("consumer canonical origin", () => {
  const base = {
    redirectURI: "http://localhost:3012/auth/callback",
    method: "GET",
    accept: "text/html,application/xhtml+xml",
  };

  it("redirects a page opened through an alternate host to the registered product origin", () => {
    expect(canonicalPageURL({
      ...base,
      requestURL: new URL("http://127.0.0.1:3012/changelog?page=2"),
    })).toBe("http://localhost:3012/changelog?page=2");
  });

  it("leaves requests on the canonical origin unchanged", () => {
    expect(canonicalPageURL({
      ...base,
      requestURL: new URL("http://localhost:3012/changelog"),
    })).toBeNull();
  });

  it.each([
    ["POST", "text/html", "/"],
    ["GET", "application/json", "/"],
    ["GET", "text/html", "/api/v1/manage"],
    ["GET", "text/html", "/auth/callback"],
    ["GET", "text/html", "/_nuxt/app.js"],
    ["GET", "text/html", "/healthz"],
  ])("does not redirect non-page or internal requests", (method, accept, path) => {
    expect(canonicalPageURL({
      ...base,
      method,
      accept,
      requestURL: new URL(`http://127.0.0.1:3012${path}`),
    })).toBeNull();
  });

  it("does nothing when the consumer has no valid registered redirect URI", () => {
    expect(canonicalPageURL({
      ...base,
      redirectURI: "",
      requestURL: new URL("http://127.0.0.1:3012/"),
    })).toBeNull();
  });
});
