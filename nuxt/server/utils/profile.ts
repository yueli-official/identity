export interface SessionDisplayUser {
  sub: string;
  email?: string;
  name?: string;
  avatar?: string;
  roles?: string[];
}

export interface PublicProfile {
  id: string;
  displayName: string;
  avatarUrl: string;
}

export interface PublicProfileResponse {
  code: string;
  data?: { profile?: PublicProfile };
}

export function createCachedProfileFetcher<T>(
  fetchProfile: (url: string) => Promise<T>,
  options: {
    ttlMs: number;
    maxEntries?: number;
    now?: () => number;
    shouldCache?: (value: T) => boolean;
  },
): (url: string) => Promise<T> {
  const now = options.now ?? Date.now;
  const maxEntries = Math.max(1, options.maxEntries ?? 500);
  const shouldCache = options.shouldCache ?? (() => true);
  const cache = new Map<string, { expiresAt: number; value: T }>();
  const inFlight = new Map<string, Promise<T>>();

  return async (url) => {
    const cached = cache.get(url);
    if (cached && cached.expiresAt > now()) {
      cache.delete(url);
      cache.set(url, cached);
      return cached.value;
    }
    if (cached) cache.delete(url);

    const pending = inFlight.get(url);
    if (pending) return pending;

    const request = fetchProfile(url)
      .then((value) => {
        if (shouldCache(value)) {
          const timestamp = now();
          for (const [key, entry] of cache) {
            if (entry.expiresAt <= timestamp) cache.delete(key);
          }
          while (cache.size >= maxEntries) {
            const oldest = cache.keys().next().value;
            if (oldest === undefined) break;
            cache.delete(oldest);
          }
          cache.set(url, {
            expiresAt: timestamp + options.ttlMs,
            value,
          });
        }
        return value;
      })
      .finally(() => {
        if (inFlight.get(url) === request) inFlight.delete(url);
      });
    inFlight.set(url, request);
    return request;
  };
}

export function mergePublicProfile<T extends SessionDisplayUser>(
  user: T,
  profile: PublicProfile,
): T {
  if (profile.id !== user.sub) return user;

  return {
    ...user,
    name: profile.displayName || user.name,
    avatar: profile.avatarUrl || undefined,
  };
}

export async function resolveLatestDisplayUser<T extends SessionDisplayUser>(
  user: T,
  issuer: string,
  fetchProfile: (url: string) => Promise<PublicProfileResponse>,
): Promise<T> {
  if (!issuer) return user;

  try {
    const response = await fetchProfile(
      `${issuer.replace(/\/$/, "")}/api/v1/profiles/${encodeURIComponent(user.sub)}`,
    );
    const profile = response.code === "ok" ? response.data?.profile : undefined;
    return profile ? mergePublicProfile(user, profile) : user;
  } catch {
    return user;
  }
}
