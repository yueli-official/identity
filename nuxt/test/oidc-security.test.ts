import {
  generateKeyPairSync,
  sign,
  type KeyObject,
} from "node:crypto";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  safeReturnTo,
  verifyIdentityIdToken,
  type OidcCfg,
} from "../server/utils/oidc";

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

    await expect(
      verifyIdentityIdToken(token, cfg, "nonce-1"),
    ).rejects.toThrow("signature is invalid");
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
