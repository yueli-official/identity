export interface PasskeyEntry {
  id: string;
  label: string;
  status: string;
  transports: string[];
  attachment: string;
  backupEligible: boolean;
  backupState: boolean;
  createdAt: string;
  lastUsedAt?: string;
}

interface BeginPasskeyRegistration {
  ceremonyId: string;
  expiresAt: string;
  options: {
    publicKey: PublicKeyCredentialCreationOptionsJSON;
  };
}

interface BeginPasskeyAuthentication {
  ceremonyId: string;
  expiresAt: string;
  options: {
    publicKey: PublicKeyCredentialRequestOptionsJSON;
  };
}

interface FinishPasskeyLogin {
  id: string;
  email: string;
}

export function passkeyErrorMessage(error: unknown): string {
  if (error instanceof DOMException) {
    if (error.name === "AbortError") return "操作已取消。";
    if (error.name === "NotAllowedError")
      return "操作已取消、超时，或此设备上没有可用的通行密钥。";
    if (error.name === "InvalidStateError")
      return "这个验证器已经为该账户保存了通行密钥。";
    if (error.name === "SecurityError")
      return "当前域名未被允许使用通行密钥，请联系管理员检查 WebAuthn 配置。";
  }
  const candidate = error as any;
  return (
    candidate?.data?.message ||
    candidate?.statusMessage ||
    "通行密钥操作失败，请重试。"
  );
}

export function usePasskeys() {
  const { call } = useApi();
  let activeCeremony: AbortController | undefined;

  onScopeDispose(() => activeCeremony?.abort());

  const isSupported = () =>
    import.meta.client &&
    "PublicKeyCredential" in window &&
    !!navigator.credentials &&
    typeof PublicKeyCredential.parseCreationOptionsFromJSON === "function" &&
    typeof PublicKeyCredential.parseRequestOptionsFromJSON === "function" &&
    typeof PublicKeyCredential.prototype.toJSON === "function";

  async function list() {
    return call<{ entries: PasskeyEntry[] }>("/api/v1/account/passkeys");
  }

  async function register(label: string) {
    if (!isSupported()) throw new Error("此浏览器不支持通行密钥。");
    activeCeremony?.abort();
    const controller = new AbortController();
    activeCeremony = controller;
    try {
      const begin = await call<BeginPasskeyRegistration>(
        "/api/v1/account/passkeys/registration/begin",
        { method: "POST", signal: controller.signal },
      );
      const credential = await navigator.credentials.create({
        publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(
          begin.options.publicKey,
        ),
        signal: controller.signal,
      });
      if (!(credential instanceof PublicKeyCredential))
        throw new Error("浏览器没有返回通行密钥。");
      return call<{ passkey: PasskeyEntry }>(
        "/api/v1/account/passkeys/registration/finish",
        {
          method: "POST",
          signal: controller.signal,
          body: {
            ceremonyId: begin.ceremonyId,
            label,
            response: credential.toJSON(),
          },
        },
      );
    } finally {
      if (activeCeremony === controller) activeCeremony = undefined;
    }
  }

  async function authenticate() {
    if (!isSupported()) throw new Error("此浏览器不支持通行密钥。");
    activeCeremony?.abort();
    const controller = new AbortController();
    activeCeremony = controller;
    try {
      const begin = await call<BeginPasskeyAuthentication>(
        "/api/v1/auth/passkeys/login/begin",
        { method: "POST", signal: controller.signal },
      );
      const credential = await navigator.credentials.get({
        publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(
          begin.options.publicKey,
        ),
        signal: controller.signal,
      });
      if (!(credential instanceof PublicKeyCredential))
        throw new Error("浏览器没有返回通行密钥。");
      return call<FinishPasskeyLogin>("/api/v1/auth/passkeys/login/finish", {
        method: "POST",
        signal: controller.signal,
        body: {
          ceremonyId: begin.ceremonyId,
          response: credential.toJSON(),
        },
      });
    } finally {
      if (activeCeremony === controller) activeCeremony = undefined;
    }
  }

  async function rename(id: string, label: string) {
    return call<{ passkey: PasskeyEntry }>(`/api/v1/account/passkeys/${id}`, {
      method: "PATCH",
      body: { label },
    });
  }

  async function remove(id: string) {
    return call(`/api/v1/account/passkeys/${id}`, { method: "DELETE" });
  }

  function cancelCeremony() {
    activeCeremony?.abort();
    activeCeremony = undefined;
  }

  return {
    isSupported,
    list,
    register,
    authenticate,
    rename,
    remove,
    cancelCeremony,
  };
}
