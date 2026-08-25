import type { OidcCfg, Session } from "./oidc";
import { compactSession, seal, unseal } from "./oidc";

const PRODUCT_SESSION_COOKIE_BUDGET_BYTES = 3500;

export interface DecodedProductSession {
  session: Session;
  needsMigration: boolean;
}

function sessionSecrets(cfg: OidcCfg): string[] {
  return [cfg.sealSecret, cfg.sealSecretPrevious]
    .map((secret) => secret?.trim() || "")
    .filter(
      (secret, index, secrets) => secret && secrets.indexOf(secret) === index,
    );
}

export function decodeProductSession(
  token: string | undefined,
  cfg: OidcCfg,
): DecodedProductSession | null {
  for (const [index, secret] of sessionSecrets(cfg).entries()) {
    const session = unseal<Session>(token, secret);
    if (
      !session?.access ||
      !session.user?.sub ||
      !Number.isFinite(session.exp)
    ) {
      continue;
    }
    const compact = compactSession(session);
    return {
      session: compact,
      needsMigration:
        index > 0 ||
        JSON.stringify(session.user) !== JSON.stringify(compact.user),
    };
  }
  return null;
}

export function encodeProductSession(session: Session, cfg: OidcCfg): string {
  const value = seal(compactSession(session), cfg.sealSecret);
  const bytes = Buffer.byteLength(`${cfg.cookies.session}=${value}`, "utf8");
  if (bytes > PRODUCT_SESSION_COOKIE_BUDGET_BYTES) {
    throw new RangeError(
      `product session cookie exceeds ${PRODUCT_SESSION_COOKIE_BUDGET_BYTES} byte budget`,
    );
  }
  return value;
}

export function decodeWithSealKeyRing<T>(
  token: string | undefined,
  cfg: OidcCfg,
): T | null {
  for (const secret of sessionSecrets(cfg)) {
    const value = unseal<T>(token, secret);
    if (value) return value;
  }
  return null;
}
