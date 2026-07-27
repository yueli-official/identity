import { describe, expect, it } from "vitest";
import { parseRecoveryCode } from "../app/composables/useMFA";

describe("parseRecoveryCode", () => {
  it("normalizes one displayed recovery code for submission", () => {
    expect(parseRecoveryCode("abcd-efgh-jklm-npqr")).toBe("ABCDEFGHJKLMNPQR");
    expect(parseRecoveryCode("ＡＢＣＤ－ＥＦＧＨ－ＪＫＬＭ－ＮＰＱＲ")).toBe(
      "ABCDEFGHJKLMNPQR",
    );
  });

  it("rejects an entire recovery-code list and invalid characters", () => {
    expect(
      parseRecoveryCode(
        "ABCD-EFGH-JKLM-NPQR\nBCDE-FGHJ-KLMN-PQRS",
      ),
    ).toBeUndefined();
    expect(parseRecoveryCode("ABCD-EFGH-JKLM-NPQ0")).toBeUndefined();
  });
});
