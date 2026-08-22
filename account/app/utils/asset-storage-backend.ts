import type { AssetStorageBackendForm } from "../types/asset-admin";

type StorageBackendConnectionDefaults = Pick<
  AssetStorageBackendForm,
  | "endpoint"
  | "region"
  | "bucketPublic"
  | "bucketPrivate"
  | "accessKey"
  | "secretKey"
  | "publicBaseUrl"
  | "pathStyle"
  | "useSsl"
>;

export function storageBackendDefaultsForType(
  type: string,
): StorageBackendConnectionDefaults {
  if (type === "cos") {
    return {
      endpoint: "",
      region: "",
      bucketPublic: "",
      bucketPrivate: "",
      accessKey: "",
      secretKey: "",
      publicBaseUrl: "",
      pathStyle: false,
      useSsl: true,
    };
  }
  if (type === "oss") {
    return {
      endpoint: "",
      region: "",
      bucketPublic: "",
      bucketPrivate: "",
      accessKey: "",
      secretKey: "",
      publicBaseUrl: "",
      pathStyle: false,
      useSsl: true,
    };
  }
  return {
    endpoint: "",
    region: "us-east-1",
    bucketPublic: "",
    bucketPrivate: "",
    accessKey: "",
    secretKey: "",
    publicBaseUrl: "",
    pathStyle: true,
    useSsl: false,
  };
}

export function normalizeStorageBackendForSubmit(
  value: AssetStorageBackendForm,
): AssetStorageBackendForm {
  if (value.type !== "cos") return { ...value };
  const bucket = value.bucketPublic.trim() || value.bucketPrivate.trim();
  return {
    ...value,
    endpoint: "",
    bucketPublic: bucket,
    bucketPrivate: bucket,
    publicBaseUrl:
      value.publicBaseUrl.trim() || cosPublicBaseURL(bucket, value.region),
    pathStyle: false,
    useSsl: true,
  };
}

export function cosEndpointForRegion(region: string) {
  const normalized = region.trim();
  return normalized ? `cos.${normalized}.myqcloud.com` : "";
}

export function cosPublicBaseURL(bucket: string, region: string) {
  const normalizedBucket = bucket.trim();
  const normalizedRegion = region.trim();
  return normalizedBucket && normalizedRegion
    ? `https://${normalizedBucket}.cos.${normalizedRegion}.myqcloud.com`
    : "";
}
