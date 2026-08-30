// Returns the current user (display claims only — never the tokens).
import {
	createCachedProfileFetcher,
	resolveLatestDisplayUser,
	type PublicUserResponse,
} from "../../utils/profile";
import { accessTokenRoles } from "../../utils/oidc";

const fetchLatestProfile = createCachedProfileFetcher(
  (url) => $fetch<PublicUserResponse>(url, { timeout: 500 }),
  {
    ttlMs: 30_000,
	shouldCache: (response) => Boolean(response.user),
  },
);

export default defineEventHandler(async (event) => {
  const cfg = oidcConfig(event);
  const hadProductSession = Boolean(getCookie(event, cfg.cookies.session));
  const s = await sessionForEvent(event, { clearOnRefreshFailure: true });
  if (!s?.user) return { user: null, reauthenticate: hadProductSession };

  try {
    await claimGuestSessionForEvent(event, s.access);
  } catch (error) {
    console.warn("guest session claim deferred", error);
  }

  const issuer = String(useRuntimeConfig(event).public.oidcIssuer || "");
  const user = await resolveLatestDisplayUser(
    { ...s.user, roles: accessTokenRoles(s.access) },
    issuer,
    fetchLatestProfile,
  );
  return { user, reauthenticate: false };
});
