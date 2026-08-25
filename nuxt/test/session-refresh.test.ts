import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  productCookieNames,
  seal,
  unseal,
  type Session,
} from "../server/utils/oidc";
import { sessionForEvent } from "../server/utils/session";

describe("OIDC BFF session refresh", () => {
  const setCookie = vi.fn();
  const deleteCookie = vi.fn();
  const runtime = {
    public: {
      oidcIssuer: "https://identity.test",
      oidcClientId: "gallery-main-web",
      oidcRedirectUri: "https://gallery.test/auth/callback",
      oidcPostLogoutRedirectUri: "https://gallery.test/",
      oidcScopes: "openid profile email roles offline_access",
    },
    oidcClientSecret: "",
    downstreamBase: "https://gallery-api.test",
    sealSecret: "session-test-secret",
    sealSecretPrevious: "previous-session-test-secret",
    cookies: productCookieNames("gallery-main-web"),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("useRuntimeConfig", () => runtime);
    vi.stubGlobal("setCookie", setCookie);
    vi.stubGlobal("deleteCookie", deleteCookie);
    vi.stubGlobal("oidcConfig", () => runtime);
    vi.stubGlobal("seal", seal);
    vi.stubGlobal("unseal", unseal);
    vi.stubGlobal("refreshSingleFlight", () => $fetch("/oauth2/token"));
    vi.stubGlobal(
      "sessionFromTokens",
      (
        tokens: { access_token: string; refresh_token?: string },
        previous: Session,
      ) => ({
        ...previous,
        access: tokens.access_token,
        refresh: tokens.refresh_token || previous.refresh,
        exp: Date.now() + 600_000,
      }),
    );
  });

  function expiredCookie(refresh?: string, secret = runtime.sealSecret) {
    const session: Session = {
      access: "expired-access",
      refresh,
      exp: Date.now() - 60_000,
      user: { sub: "user-1", name: "测试用户", roles: ["admin"] },
    };
    return seal(session, secret);
  }

  it("does not erase the login cookie when Identity is temporarily unavailable", async () => {
    vi.stubGlobal("getCookie", () => expiredCookie("refresh-1"));
    vi.stubGlobal(
      "$fetch",
      vi
        .fn()
        .mockRejectedValue(
          Object.assign(new Error("network unavailable"), { statusCode: 503 }),
        ),
    );

    await expect(
      sessionForEvent({} as never, { clearOnRefreshFailure: true }),
    ).rejects.toThrow("network unavailable");
    expect(deleteCookie).not.toHaveBeenCalled();
  });

  it("renews and rotates an expired session without asking the user to log in", async () => {
    vi.stubGlobal("getCookie", () => expiredCookie("refresh-1"));
    vi.stubGlobal(
      "$fetch",
      vi.fn().mockResolvedValue({
        access_token: "fresh-access",
        refresh_token: "refresh-2",
        expires_in: 600,
      }),
    );

    await expect(sessionForEvent({} as never)).resolves.toMatchObject({
      access: "fresh-access",
      refresh: "refresh-2",
      user: { sub: "user-1" },
    });
    expect(setCookie).toHaveBeenCalledWith(
      expect.anything(),
      runtime.cookies.session,
      expect.any(String),
      expect.objectContaining({ maxAge: 7 * 24 * 60 * 60 }),
    );
    expect(deleteCookie).not.toHaveBeenCalled();
  });

  it("renews a session sealed with the previous key and migrates it to the current key", async () => {
    vi.stubGlobal("getCookie", () =>
      expiredCookie("refresh-1", runtime.sealSecretPrevious),
    );
    vi.stubGlobal(
      "$fetch",
      vi.fn().mockResolvedValue({
        access_token: "fresh-access",
        refresh_token: "refresh-2",
        expires_in: 600,
      }),
    );

    await expect(sessionForEvent({} as never)).resolves.toMatchObject({
      access: "fresh-access",
      refresh: "refresh-2",
    });
    const migrated = setCookie.mock.calls.at(-1)?.[2];
    expect(unseal<Session>(migrated, runtime.sealSecret)).toMatchObject({
      access: "fresh-access",
      refresh: "refresh-2",
    });
  });

  it("clears a refresh token only when Identity says it is permanently invalid", async () => {
    vi.stubGlobal("getCookie", () => expiredCookie("revoked-refresh"));
    vi.stubGlobal(
      "$fetch",
      vi.fn().mockRejectedValue(
        Object.assign(new Error("invalid_grant"), {
          statusCode: 400,
          data: { error: "invalid_grant" },
        }),
      ),
    );

    await expect(
      sessionForEvent({} as never, { clearOnRefreshFailure: true }),
    ).resolves.toBeNull();
    expect(deleteCookie).toHaveBeenCalledOnce();
  });

  it("does not forward an expired access token when no refresh token exists", async () => {
    vi.stubGlobal("getCookie", () => expiredCookie());
    vi.stubGlobal("$fetch", vi.fn());

    await expect(
      sessionForEvent({} as never, { clearOnRefreshFailure: true }),
    ).resolves.toBeNull();
    expect(deleteCookie).toHaveBeenCalledOnce();
    expect($fetch).not.toHaveBeenCalled();
  });
});
