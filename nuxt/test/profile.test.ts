import { describe, expect, it, vi } from "vitest";
import {
  createCachedProfileFetcher,
  mergePublicProfile,
  resolveLatestDisplayUser,
} from "../server/utils/profile";

describe("mergePublicProfile", () => {
  it("uses the latest Identity display name and avatar", () => {
    expect(
      mergePublicProfile(
        {
          sub: "user-1",
          email: "old@example.com",
          name: "旧名称",
          avatar: "https://old.example/avatar.png",
          roles: ["member"],
        },
        {
          id: "user-1",
          displayName: "新名称",
          avatarUrl: "https://identity.example/avatar.png",
        },
      ),
    ).toEqual({
      sub: "user-1",
      email: "old@example.com",
      name: "新名称",
      avatar: "https://identity.example/avatar.png",
      roles: ["member"],
    });
  });

  it("clears a removed avatar without dropping private session fields", () => {
    expect(
      mergePublicProfile(
        {
          sub: "user-1",
          email: "user@example.com",
          avatar: "https://old.example/avatar.png",
        },
        { id: "user-1", displayName: "", avatarUrl: "" },
      ),
    ).toEqual({
      sub: "user-1",
      email: "user@example.com",
      avatar: undefined,
    });
  });

  it("ignores a profile for a different identity", () => {
    const user = { sub: "user-1", name: "当前用户" };
    expect(
      mergePublicProfile(user, {
        id: "user-2",
        displayName: "其他用户",
        avatarUrl: "https://example.com/other.png",
      }),
    ).toEqual(user);
  });

  it("resolves the current profile from the configured Identity issuer", async () => {
    const fetchProfile = vi.fn().mockResolvedValue({
      code: "ok",
      data: {
        profile: {
          id: "user/1",
          displayName: "月离",
          avatarUrl: "https://identity.example/avatar.png",
        },
      },
    });

    await expect(
      resolveLatestDisplayUser(
        { sub: "user/1", name: "旧名称" },
        "https://identity.example/",
        fetchProfile,
      ),
    ).resolves.toMatchObject({
      name: "月离",
      avatar: "https://identity.example/avatar.png",
    });
    expect(fetchProfile).toHaveBeenCalledWith(
      "https://identity.example/api/v1/profiles/user%2F1",
    );
  });

  it("keeps the sealed session user when Identity is unavailable", async () => {
    const user = { sub: "user-1", name: "会话名称", avatar: "old.png" };
    await expect(
      resolveLatestDisplayUser(user, "https://identity.example", async () => {
        throw new Error("offline");
      }),
    ).resolves.toEqual(user);
  });
});

describe("createCachedProfileFetcher", () => {
  it("reuses a recent profile response and coalesces concurrent requests", async () => {
    let release: ((value: { code: string }) => void) | undefined;
    const fetchProfile = vi.fn(
      () =>
        new Promise<{ code: string }>((resolve) => {
          release = resolve;
        }),
    );
    const cachedFetch = createCachedProfileFetcher(fetchProfile, {
      ttlMs: 30_000,
      now: () => 1_000,
    });

    const first = cachedFetch("https://identity.example/profile/user-1");
    const concurrent = cachedFetch("https://identity.example/profile/user-1");
    release?.({ code: "ok" });

    await expect(Promise.all([first, concurrent])).resolves.toEqual([
      { code: "ok" },
      { code: "ok" },
    ]);
    await expect(
      cachedFetch("https://identity.example/profile/user-1"),
    ).resolves.toEqual({ code: "ok" });
    expect(fetchProfile).toHaveBeenCalledOnce();
  });

  it("retries after the cache expires", async () => {
    let now = 1_000;
    const fetchProfile = vi.fn().mockResolvedValue({ code: "ok" });
    const cachedFetch = createCachedProfileFetcher(fetchProfile, {
      ttlMs: 100,
      now: () => now,
    });

    await cachedFetch("https://identity.example/profile/user-1");
    now = 1_101;
    await cachedFetch("https://identity.example/profile/user-1");

    expect(fetchProfile).toHaveBeenCalledTimes(2);
  });

  it("bounds retained profiles with least-recently-used eviction", async () => {
    const fetchProfile = vi.fn(async (url: string) => ({ code: "ok", url }));
    const cachedFetch = createCachedProfileFetcher(fetchProfile, {
      ttlMs: 30_000,
      maxEntries: 2,
      now: () => 1_000,
    });

    await cachedFetch("profile-a");
    await cachedFetch("profile-b");
    await cachedFetch("profile-c");
    await cachedFetch("profile-a");

    expect(fetchProfile).toHaveBeenCalledTimes(4);
  });
});
