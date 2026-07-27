import { beforeEach, describe, expect, it, vi } from "vitest";

import { guestSessionAuthHeaders } from "../server/utils/guest";

describe("guestSessionAuthHeaders", () => {
  const setCookie = vi.fn();
  const deleteCookie = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("useRuntimeConfig", () => ({
      guestSessionTtlSeconds: 30 * 24 * 60 * 60,
      guestCookieSecure: false,
      public: {
        oidcIssuer: "http://identity.test",
        oidcClientId: "gallery-main-web",
      },
    }));
    // H3 writes cookie changes to the response; the current request cookie is
    // immutable and remains visible for the rest of this event.
    vi.stubGlobal("getCookie", () => "expired-session");
    vi.stubGlobal("setCookie", setCookie);
    vi.stubGlobal("deleteCookie", deleteCookie);
    vi.stubGlobal("createError", (value: unknown) => value);
  });

  it("replaces an invalid durable cookie and retries token issuance once", async () => {
    const fetch = vi
      .fn()
      .mockRejectedValueOnce(
        Object.assign(new Error("identity.guest_session_invalid"), {
          failure: {
            kind: "remote",
            status: 401,
            code: "identity.guest_session_invalid",
            params: {},
            violations: [],
            traceId: "guest-expired",
            reauth: "not-attempted",
          },
        }),
      )
      .mockResolvedValueOnce({
        subjectId: "guest-2",
        sessionToken: "replacement-session",
        effectiveTtlSeconds: 30 * 24 * 60 * 60,
        expiresAt: "2026-08-16T00:00:00Z",
      })
      .mockResolvedValueOnce({
        accessToken: "replacement-access",
        expiresInSeconds: 600,
      });
    vi.stubGlobal("$fetch", fetch);

    await expect(
      guestSessionAuthHeaders({} as never, "asset-api", false),
    ).resolves.toEqual({ authorization: "Bearer replacement-access" });
    expect(deleteCookie).toHaveBeenCalledOnce();
    expect(setCookie).toHaveBeenCalledWith(
      expect.anything(),
      "yueli_guest",
      "replacement-session",
      expect.objectContaining({ httpOnly: true }),
    );
    expect(fetch).toHaveBeenCalledTimes(3);
  });
});
