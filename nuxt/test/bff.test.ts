import { describe, expect, it } from "vitest";
import {
  identityBffCredential,
  identityBffTarget,
} from "../server/utils/bff";

describe("Identity Nuxt BFF adapter", () => {
  it("splits a configured private base into origin and fixed path", () => {
    expect(identityBffTarget("https://shop.internal/v1")).toEqual({
      origin: "https://shop.internal",
      pathPrefix: "/v1/api/v1",
    });
    expect(identityBffTarget("https://shop.internal")).toEqual({
      origin: "https://shop.internal",
      pathPrefix: "/api/v1",
    });
  });

  it("rejects credentials and query data in a configured target", () => {
    expect(() =>
      identityBffTarget("https://user:pass@shop.internal"),
    ).toThrow("Invalid private BFF target");
    expect(() =>
      identityBffTarget("https://shop.internal?target=evil"),
    ).toThrow("Invalid private BFF target");
  });

  it("forwards only an explicit bearer credential", () => {
    expect(
      identityBffCredential({ authorization: "Bearer internal-token" }),
    ).toEqual({ kind: "bearer", token: "internal-token" });
    expect(identityBffCredential({ cookie: "browser=secret" })).toEqual({
      kind: "anonymous",
    });
  });
});
