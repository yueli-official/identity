import { generateKeyPairSync, sign, type KeyObject } from "node:crypto";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  accessTokenRoles,
  refreshSingleFlight,
  safeReturnTo,
  seal,
  sessionFromTokens,
  verifyIdentityIdToken,
  type OidcCfg,
} from "../server/utils/oidc";
import { encodeProductSession } from "../server/utils/product-session";

function encode(value: unknown): string {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
}

function signedToken(
  privateKey: KeyObject,
  claims: Record<string, unknown>,
  kid = "identity-key-1",
): string {
  const header = encode({ alg: "RS256", kid, typ: "JWT" });
  const payload = encode(claims);
  const signature = sign(
    "RSA-SHA256",
    Buffer.from(`${header}.${payload}`),
    privateKey,
  ).toString("base64url");
  return `${header}.${payload}.${signature}`;
}

describe("accessTokenRoles", () => {
  it("projects bounded unique role claims without storing them in the product session user", () => {
    const token = `${encode({ alg: "RS256" })}.${encode({
      roles: ["admin", "user", "admin", "", 42],
    })}.signature`;

    expect(accessTokenRoles(token)).toEqual(["admin", "user"]);
    expect(accessTokenRoles("not-a-jwt")).toEqual([]);
  });
});

function config(suffix: string): OidcCfg {
  const issuer = `https://identity-${suffix}.test`;
  return {
    issuer,
    clientId: "shop-main-web",
    clientSecret: "",
    redirectUri: "https://shop.test/auth/callback",
    postLogoutRedirectUri: "https://shop.test/",
    scopes: "openid profile email roles offline_access",
    authorizeEndpoint: `${issuer}/oauth2/authorize`,
    tokenEndpoint: `${issuer}/oauth2/token`,
    jwksEndpoint: `${issuer}/oauth2/jwks.json`,
    endSessionEndpoint: `${issuer}/oauth2/end_session`,
    downstreamBase: "https://shop-api.test",
    sealSecret: "test-seal-secret-at-least-32-bytes",
    cookieSecure: true,
    cookies: {
      session: "ys_shop-main_0123456789ab",
      transaction: "yt_shop-main_0123456789ab",
    },
  };
}

describe("verifyIdentityIdToken", () => {
  const { publicKey, privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
  });
  const jwk = {
    ...publicKey.export({ format: "jwk" }),
    alg: "RS256",
    kid: "identity-key-1",
    use: "sig",
  };

  beforeEach(() => {
    vi.stubGlobal("$fetch", vi.fn().mockResolvedValue({ keys: [jwk] }));
  });

  it("accepts a signed token for the exact issuer, audience and nonce", async () => {
    const cfg = config("valid");
    const token = signedToken(privateKey, {
      iss: cfg.issuer,
      sub: "user-1",
      aud: cfg.clientId,
      exp: Math.floor(Date.now() / 1000) + 300,
      nonce: "nonce-1",
      name: "月离",
    });

    await expect(
      verifyIdentityIdToken(token, cfg, "nonce-1"),
    ).resolves.toMatchObject({ sub: "user-1", name: "月离" });
  });

  it("rejects a valid signature when nonce binding fails", async () => {
    const cfg = config("nonce");
    const token = signedToken(privateKey, {
      iss: cfg.issuer,
      sub: "user-1",
      aud: cfg.clientId,
      exp: Math.floor(Date.now() / 1000) + 300,
      nonce: "nonce-from-another-flow",
    });

    await expect(
      verifyIdentityIdToken(token, cfg, "expected-nonce"),
    ).rejects.toThrow("claims are invalid");
  });

  it("rejects an ID Token signed by an unknown key", async () => {
    const cfg = config("signature");
    const attacker = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const token = signedToken(attacker.privateKey, {
      iss: cfg.issuer,
      sub: "user-1",
      aud: cfg.clientId,
      exp: Math.floor(Date.now() / 1000) + 300,
      nonce: "nonce-1",
    });

    await expect(verifyIdentityIdToken(token, cfg, "nonce-1")).rejects.toThrow(
      "signature is invalid",
    );
  });
});

describe("sessionFromTokens", () => {
  it("keeps the pairwise subject and exposes the stable public user key", () => {
    const session = sessionFromTokens(
      { access_token: "access", expires_in: 600 },
      undefined,
      {
        iss: "https://identity.test",
        sub: "pairwise-subject",
        user_key: "TestA123",
        aud: "nav-yueli-web",
        exp: Math.floor(Date.now() / 1000) + 300,
        nonce: "nonce",
      },
    );
    expect(session.user).toMatchObject({
      sub: "pairwise-subject",
      userKey: "TestA123",
    });
  });

  it("keeps the sealed product session within budget when mutable profile claims are oversized", () => {
    const session = sessionFromTokens(
      {
        access_token: "a".repeat(1400),
        refresh_token: "r".repeat(96),
        expires_in: 600,
      },
      undefined,
      {
        iss: "https://identity.test",
        sub: "pairwise-subject",
        user_key: "TestA123",
        aud: "blog-main-web",
        exp: Math.floor(Date.now() / 1000) + 300,
        nonce: "nonce",
        email: `${"e".repeat(600)}@example.com`,
        name: "名".repeat(1000),
        picture: `https://identity.test/${"avatar/".repeat(400)}`,
        roles: Array.from(
          { length: 100 },
          (_, index) => `role-${index}-${"x".repeat(40)}`,
        ),
      },
    );
    const value = seal(session, "test-seal-secret-at-least-32-bytes");

    expect(
      Buffer.byteLength(`ys_blog-main_0123456789ab=${value}`),
    ).toBeLessThanOrEqual(3500);
    expect(session.user.name?.length).toBeLessThanOrEqual(160);
    expect(session.user.email?.length).toBeLessThanOrEqual(320);
    expect(session.user).not.toHaveProperty("avatar");
    expect(session.user).not.toHaveProperty("roles");
  });

  it("rejects a product session before the browser can silently discard an oversized cookie", () => {
    expect(() =>
      encodeProductSession(
        {
          access: "a".repeat(4000),
          refresh: "refresh",
          exp: Date.now() + 600_000,
          user: { sub: "user-1", userKey: "TestA123" },
        },
        config("cookie-budget"),
      ),
    ).toThrow(/3500 byte budget/);
  });
});

describe("refreshSingleFlight", () => {
  it("reuses a freshly rotated result for a late request carrying the stale cookie", async () => {
    const fetchToken = vi.fn().mockResolvedValue({
      access_token: "fresh-access",
      refresh_token: "rotated-refresh",
      expires_in: 600,
    });
    vi.stubGlobal("$fetch", fetchToken);
    const cfg = config(`late-refresh-${Date.now()}`);
    const staleRefresh = `stale-refresh-${Date.now()}`;

    await expect(refreshSingleFlight(cfg, staleRefresh)).resolves.toMatchObject({
      refresh_token: "rotated-refresh",
    });
    await expect(refreshSingleFlight(cfg, staleRefresh)).resolves.toMatchObject({
      refresh_token: "rotated-refresh",
    });

    expect(fetchToken).toHaveBeenCalledTimes(1);
  });
});

describe("safeReturnTo", () => {
  it("keeps an ordinary same-origin relative path", () => {
    expect(safeReturnTo("/orders?status=paid")).toBe("/orders?status=paid");
  });

  it.each([
    "https://evil.test",
    "//evil.test",
    "/%2F%2Fevil.test",
    "/%255C%255Cevil.test",
    "/orders%0d%0aLocation:%20https://evil.test",
  ])("rejects unsafe or layered redirect input %s", (value) => {
    expect(safeReturnTo(value)).toBe("/");
  });
});
