import { describe, expect, it } from "vitest";

import { productCookieNames } from "../server/utils/oidc";

describe("productCookieNames", () => {
  it("is stable and diagnostically readable for one OIDC client", () => {
    const first = productCookieNames("blog-main-web");
    const second = productCookieNames("blog-main-web");

    expect(first).toEqual(second);
    expect(first.session).toMatch(
      /^ys_blog-main_[0-9a-f]{12}$/,
    );
    expect(first.transaction).toMatch(
      /^yt_blog-main_[0-9a-f]{12}$/,
    );
  });

  it("isolates two instances of the same product kind", () => {
    const firstBlog = productCookieNames("blog-main-web");
    const secondBlog = productCookieNames("blog-team-web");

    expect(firstBlog.session).not.toBe(secondBlog.session);
    expect(firstBlog.transaction).not.toBe(secondBlog.transaction);
    expect(firstBlog.session).not.toBe(firstBlog.transaction);
  });

  it("refuses an empty client identity instead of falling back to a shared cookie", () => {
    expect(() => productCookieNames("")).toThrow(/client/i);
  });

  it("keeps a readable safe label without relying on it for uniqueness", () => {
    const first = productCookieNames("Docs 主站/client");
    const second = productCookieNames("docs___client");

    expect(first.session).toMatch(
      /^ys_docs-client_[0-9a-f]{12}$/,
    );
    expect(second.session).toMatch(
      /^ys_docs-client_[0-9a-f]{12}$/,
    );
    expect(first.session).not.toBe(second.session);
  });
});
