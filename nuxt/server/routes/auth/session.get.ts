// Returns the current user (display claims only — never the tokens).
import {
  createCachedProfileFetcher,
  resolveLatestDisplayUser,
  type PublicProfileResponse,
} from "../../utils/profile";

const fetchLatestProfile = createCachedProfileFetcher(
  (url) => $fetch<PublicProfileResponse>(url, { timeout: 500 }),
  {
    ttlMs: 30_000,
    shouldCache: (response) => response.code === "ok",
  },
);

export default defineEventHandler(async (event) => {
  const s = await sessionForEvent(event, { clearOnRefreshFailure: true });
  if (!s?.user) return { user: null };

  try {
    await claimGuestSessionForEvent(event, s.access);
  } catch (error) {
    console.warn("guest session claim deferred", error);
  }

  const issuer = String(useRuntimeConfig(event).public.oidcIssuer || "");
  const user = await resolveLatestDisplayUser(
    s.user,
    issuer,
    fetchLatestProfile,
  );
  return { user };
});
