export interface PasskeyEntry {
  id: string
  label: string
  status: string
  transports: string[]
  attachment: string
  backupEligible: boolean
  backupState: boolean
  createdAt: string
  lastUsedAt?: string
}

interface BeginPasskeyCeremony {
  ceremonyId: string
  expiresAt: string
  options: {
    publicKey: Record<string, any>
  }
}

interface FinishPasskeyLogin {
  id: string
  email: string
}

const base64UrlToBytes = (value: string): ArrayBuffer => {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index)
  return bytes.buffer
}

const bytesToBase64Url = (value: ArrayBuffer | ArrayBufferView): string => {
  const bytes = value instanceof ArrayBuffer
    ? new Uint8Array(value)
    : new Uint8Array(value.buffer, value.byteOffset, value.byteLength)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function decodeCreationOptions(options: BeginPasskeyCeremony['options']): CredentialCreationOptions {
  const publicKey = options.publicKey
  return {
    publicKey: {
      ...publicKey,
      challenge: base64UrlToBytes(publicKey.challenge),
      user: {
        ...publicKey.user,
        id: base64UrlToBytes(publicKey.user.id),
      },
      excludeCredentials: (publicKey.excludeCredentials ?? []).map((credential: any) => ({
        ...credential,
        id: base64UrlToBytes(credential.id),
      })),
    } as PublicKeyCredentialCreationOptions,
  }
}

function decodeRequestOptions(options: BeginPasskeyCeremony['options']): CredentialRequestOptions {
  const publicKey = options.publicKey
  return {
    publicKey: {
      ...publicKey,
      challenge: base64UrlToBytes(publicKey.challenge),
      allowCredentials: (publicKey.allowCredentials ?? []).map((credential: any) => ({
        ...credential,
        id: base64UrlToBytes(credential.id),
      })),
    } as PublicKeyCredentialRequestOptions,
  }
}

function extensionResultsToJSON(value: unknown): unknown {
  if (value instanceof ArrayBuffer || ArrayBuffer.isView(value)) return bytesToBase64Url(value)
  if (Array.isArray(value)) return value.map(extensionResultsToJSON)
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, extensionResultsToJSON(item)]),
    )
  }
  return value
}

function serializeCredential(credential: PublicKeyCredential) {
  const response = credential.response
  const common = {
    id: credential.id,
    rawId: bytesToBase64Url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: extensionResultsToJSON(credential.getClientExtensionResults()),
  }
  if ('attestationObject' in response) {
    const attestation = response as AuthenticatorAttestationResponse
    return {
      ...common,
      response: {
        clientDataJSON: bytesToBase64Url(attestation.clientDataJSON),
        attestationObject: bytesToBase64Url(attestation.attestationObject),
        transports: attestation.getTransports?.() ?? [],
      },
    }
  }
  const assertion = response as AuthenticatorAssertionResponse
  return {
    ...common,
    response: {
      clientDataJSON: bytesToBase64Url(assertion.clientDataJSON),
      authenticatorData: bytesToBase64Url(assertion.authenticatorData),
      signature: bytesToBase64Url(assertion.signature),
      userHandle: assertion.userHandle ? bytesToBase64Url(assertion.userHandle) : null,
    },
  }
}

export function passkeyErrorMessage(error: unknown): string {
  if (error instanceof DOMException) {
    if (error.name === 'NotAllowedError') return '操作已取消、超时，或此设备上没有可用的通行密钥。'
    if (error.name === 'InvalidStateError') return '这个验证器已经为该账户保存了通行密钥。'
    if (error.name === 'SecurityError') return '当前域名未被允许使用通行密钥，请联系管理员检查 WebAuthn 配置。'
  }
  const candidate = error as any
  return candidate?.data?.message || candidate?.statusMessage || '通行密钥操作失败，请重试。'
}

export function usePasskeys() {
  const { call } = useApi()

  const isSupported = () => import.meta.client &&
    'PublicKeyCredential' in window &&
    !!navigator.credentials

  async function list() {
    return call<{ entries: PasskeyEntry[] }>('/api/v1/account/passkeys')
  }

  async function register(label: string) {
    if (!isSupported()) throw new Error('此浏览器不支持通行密钥。')
    const begin = await call<BeginPasskeyCeremony>(
      '/api/v1/account/passkeys/registration/begin',
      { method: 'POST' },
    )
    const credential = await navigator.credentials.create(decodeCreationOptions(begin.options))
    if (!(credential instanceof PublicKeyCredential)) throw new Error('浏览器没有返回通行密钥。')
    return call<{ passkey: PasskeyEntry }>(
      '/api/v1/account/passkeys/registration/finish',
      {
        method: 'POST',
        body: {
          ceremonyId: begin.ceremonyId,
          label,
          response: serializeCredential(credential),
        },
      },
    )
  }

  async function authenticate() {
    if (!isSupported()) throw new Error('此浏览器不支持通行密钥。')
    const begin = await call<BeginPasskeyCeremony>(
      '/api/v1/auth/passkeys/login/begin',
      { method: 'POST' },
    )
    const credential = await navigator.credentials.get(decodeRequestOptions(begin.options))
    if (!(credential instanceof PublicKeyCredential)) throw new Error('浏览器没有返回通行密钥。')
    return call<FinishPasskeyLogin>(
      '/api/v1/auth/passkeys/login/finish',
      {
        method: 'POST',
        body: {
          ceremonyId: begin.ceremonyId,
          response: serializeCredential(credential),
        },
      },
    )
  }

  async function rename(id: string, label: string) {
    return call<{ passkey: PasskeyEntry }>(`/api/v1/account/passkeys/${id}`, {
      method: 'PATCH',
      body: { label },
    })
  }

  async function remove(id: string) {
    return call(`/api/v1/account/passkeys/${id}`, { method: 'DELETE' })
  }

  return { isSupported, list, register, authenticate, rename, remove }
}
