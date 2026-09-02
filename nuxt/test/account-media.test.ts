import { describe, expect, it } from "vitest";
import { accountMediaUrl } from "../app/utils/account-media";

describe("consumer account media URLs", () => {
  it("routes account-owned media through the Account origin", () => {
    expect(
      accountMediaUrl(
        "/media/avatar-key?format=webp&name=thumbnail&v=1",
        "https://accounts.example.com",
      ),
    ).toBe(
      "https://accounts.example.com/media/avatar-key?format=webp&name=thumbnail&v=1",
    );
  });

  it("preserves absolute and empty values", () => {
    expect(accountMediaUrl("https://cdn.example.com/avatar.webp", "https://accounts.example.com"))
      .toBe("https://cdn.example.com/avatar.webp");
    expect(accountMediaUrl(undefined, "https://accounts.example.com")).toBe("");
  });
});
