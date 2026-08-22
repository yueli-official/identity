import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import {
  normalizeStorageBackendForSubmit,
  storageBackendDefaultsForType,
} from "../app/utils/asset-storage-backend";

describe("Tencent COS storage backend form", () => {
  it("uses COS-safe defaults instead of inheriting MinIO values", () => {
    expect(storageBackendDefaultsForType("cos")).toEqual({
      endpoint: "",
      region: "",
      bucketPublic: "",
      bucketPrivate: "",
      accessKey: "",
      secretKey: "",
      publicBaseUrl: "",
      pathStyle: false,
      useSsl: true,
    });
  });

  it("maps one COS bucket to both Asset storage roles", () => {
    expect(
      normalizeStorageBackendForSubmit({
        name: "tencent-cos",
        type: "cos",
        enabled: true,
        endpoint: "localhost:9000",
        region: "ap-shanghai",
        bucketPublic: "blog-1300000000",
        bucketPrivate: "",
        accessKey: "secret-id",
        secretKey: "secret-key",
        publicBaseUrl: "",
        pathStyle: true,
        useSsl: false,
      }),
    ).toMatchObject({
      endpoint: "",
      bucketPublic: "blog-1300000000",
      bucketPrivate: "blog-1300000000",
      publicBaseUrl:
        "https://blog-1300000000.cos.ap-shanghai.myqcloud.com",
      pathStyle: false,
      useSsl: true,
    });
  });

  it("renders one COS bucket and does not expose MinIO-only controls", () => {
    const source = readFileSync(
      new URL("../app/components/AssetStorageBackendModal.vue", import.meta.url),
      "utf8",
    );
    expect(source).toMatch(/<UFormField\s+v-if="isCOS"\s+label="Bucket"/u);
    expect(source).toMatch(
      /<UFormField\s+v-if="!isCOS"\s+label="Public Bucket"/u,
    );
    expect(source).toMatch(
      /<UFormField\s+v-if="!isCOS"\s+label="Private Bucket"/u,
    );
    expect(source).toMatch(
      /<UFormField\s+v-if="!isCOS"\s+label="Endpoint"/u,
    );
    expect(source).toMatch(
      /<UCheckbox\s+v-if="!isCOS"[\s\S]*?label="Path-style endpoint"/u,
    );
    expect(source).toContain("{ flush: 'sync' }");
  });
});
