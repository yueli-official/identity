import { computed, ref } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuth } from "../app/composables/useAuth";

describe("useAuth transient recovery", () => {
  const user = ref(null);
  const reauthenticate = ref(false);

  beforeEach(() => {
    vi.useFakeTimers();
    user.value = null;
    reauthenticate.value = false;
    vi.stubGlobal("useRuntimeConfig", () => ({ public: { operatorSubs: "" } }));
    vi.stubGlobal("useState", (key: string) => key === "rs-auth-reauthenticate" ? reauthenticate : user);
    vi.stubGlobal("useRequestHeaders", () => undefined);
    vi.stubGlobal("computed", computed);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("retries a transient session failure and restores the confirmed user", async () => {
    const fetch = vi
      .fn()
      .mockRejectedValueOnce(Object.assign(new Error("temporary outage"), { statusCode: 503 }))
      .mockResolvedValueOnce({ user: { sub: "user-1", name: "测试用户" } });
    vi.stubGlobal("$fetch", fetch);

    const { refresh, loggedIn } = useAuth();
    await expect(refresh()).resolves.toBeNull();
    expect(loggedIn.value).toBe(false);
    expect(fetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1_000);

    expect(fetch).toHaveBeenCalledTimes(2);
    expect(loggedIn.value).toBe(true);
  });

  it("silently rebuilds an invalid product session through central SSO", async () => {
    const fetch = vi.fn().mockResolvedValue({ user: null, reauthenticate: true });
    const navigateTo = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("$fetch", fetch);
    vi.stubGlobal("navigateTo", navigateTo);
    window.history.replaceState({}, "", "/guide?section=api");

    const { refresh } = useAuth();
    await expect(refresh()).resolves.toBeNull();

    expect(navigateTo).toHaveBeenCalledWith(
      "/auth/login?return_to=%2Fguide%3Fsection%3Dapi",
      { external: true },
    );
  });

  it("preserves the SSR recovery intent when the browser no longer has the deleted cookie", async () => {
    reauthenticate.value = true;
    const fetch = vi.fn().mockResolvedValue({ user: null, reauthenticate: false });
    const navigateTo = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("$fetch", fetch);
    vi.stubGlobal("navigateTo", navigateTo);
    window.history.replaceState({}, "", "/guide");

    const { refresh } = useAuth();
    await refresh();

    expect(navigateTo).toHaveBeenCalledWith(
      "/auth/login?return_to=%2Fguide",
      { external: true },
    );
  });

  it("keeps a genuinely anonymous public visit anonymous", async () => {
    const fetch = vi.fn().mockResolvedValue({ user: null, reauthenticate: false });
    const navigateTo = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("$fetch", fetch);
    vi.stubGlobal("navigateTo", navigateTo);

    const { refresh, loggedIn } = useAuth();
    await expect(refresh()).resolves.toBeNull();

    expect(loggedIn.value).toBe(false);
    expect(navigateTo).not.toHaveBeenCalled();
  });

});
