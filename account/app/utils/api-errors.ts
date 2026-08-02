import {
  getApiFailure,
  type ProblemParams,
  type RemoteFailure,
} from '@yueli/http-runtime'

export type IdentityErrorContext =
  | 'generic'
  | 'login'
  | 'register'
  | 'password-change'
  | 'password-reset'
  | 'password-set'
  | 'profile'
  | 'session'
  | 'credential'
  | 'passkey'
  | 'mfa'
  | 'verification'
  | 'admin'

export interface ErrorMessageOptions {
  context?: IdentityErrorContext
  fallback?: string
}

export const IDENTITY_ERROR_MESSAGES = {
  'identity.abuse_attempt_replayed': '该请求已经处理，请刷新页面确认当前状态。',
  'identity.abuse_unavailable': '安全校验服务暂时不可用，请稍后再试。',
  'identity.account_disabled': '该账户当前不可用；如需帮助，请联系支持。',
  'identity.account_locked': '尝试次数过多，请稍后再试。',
  'identity.capability_audit_unavailable': '服务能力审计暂时不可用，请稍后再试。',
  'identity.capability_not_found': '没有找到指定的服务能力。',
  'identity.capability_probe_rate_limited': '健康检查过于频繁，请稍后再试。',
  'identity.challenge_required': '请先完成安全验证，然后继续。',
  'identity.credential_conflict': '该登录账户已经绑定到其他用户。',
  'identity.credential_not_found': '没有找到这项登录方式，请刷新后确认当前状态。',
  'identity.email_taken': '该邮箱已注册，请直接登录或找回密码。',
  'identity.forbidden': '你没有执行此操作的权限。',
  'identity.github_binding_attempt_invalid': 'GitHub 绑定请求无效、已过期或已经使用，请重新开始。',
  'identity.github_binding_conflict': '该 GitHub 账户已经绑定到其他用户。',
  'identity.github_binding_not_found': '没有找到该 GitHub 绑定，请刷新后确认当前状态。',
  'identity.github_binding_unavailable': 'GitHub 绑定服务暂时不可用，请稍后再试。',
  'identity.github_provider_failed': '无法验证 GitHub 账户，请稍后再试。',
  'identity.github_submission_invalid': 'GitHub 提交清单无效，请检查后重新提交。',
  'identity.github_submission_unauthorized': '当前 GitHub 身份无权为该发布者提交内容。',
  'identity.guest_audience_invalid': '该访客会话不能用于当前站点。',
  'identity.guest_claim_conflict': '该访客会话已经归属其他账户。',
  'identity.guest_request_invalid': '访客会话请求无效，请刷新后重试。',
  'identity.guest_session_invalid': '访客会话无效或已过期，请刷新后重试。',
  'identity.handle_unavailable': '该 Handle 已被占用或保留，请换一个再试。',
  'identity.invalid_credentials': '邮箱或密码不正确。',
  'identity.invalid_email': '邮箱格式不正确，请检查后重新输入。',
  'identity.invalid_profile': '账户资料不符合要求，请检查后重新提交。',
  'identity.invalid_status': '账户状态值无效。',
  'identity.last_credential': '这是当前唯一登录方式；请先添加另一种登录方式。',
  'identity.mfa_transaction_invalid': '登录验证已过期，请重新登录。',
  'identity.mfa_unavailable': '身份验证器服务暂时不可用，请稍后再试。',
  'identity.not_authenticated': '登录状态已失效，请重新登录。',
  'identity.not_found': '没有找到该用户，请刷新后确认。',
  'identity.oauth_email_conflict': '该邮箱已有账户，请先用原有方式登录后再绑定。',
  'identity.oauth_failed': '第三方登录失败，请重试或改用其他登录方式。',
  'identity.oauth_no_email': '第三方账户没有提供可用邮箱，请改用其他登录方式。',
  'identity.passkey_ceremony_invalid': '这枚通行密钥当前无法用于登录。它可能已被移除，或本次请求已过期；请改用其他登录方式。',
  'identity.passkey_exists': '这个验证器已经为该账户保存了通行密钥。',
  'identity.passkey_unavailable': '通行密钥服务暂时不可用，请稍后再试或改用密码登录。',
  'identity.password_already_set': '该账户已有密码，请使用“修改密码”。',
  'identity.pat_expired': '访问令牌已过期，请创建新令牌。',
  'identity.pat_invalid': '访问令牌无效。',
  'identity.pat_limit_reached': '访问令牌数量已达上限，请先移除不再使用的令牌。',
  'identity.pat_name_required': '请填写访问令牌名称。',
  'identity.pat_not_found': '没有找到该访问令牌，请刷新后确认。',
  'identity.pat_scope_invalid': '访问令牌包含无效权限范围，请检查后重新提交。',
  'identity.pat_scopes_required': '请至少选择一个访问令牌权限范围。',
  'identity.provider_not_found': '没有找到指定的身份提供方。',
  'identity.publisher_attestation_invalid': '发布者证明请求无效，请检查后重新提交。',
  'identity.publisher_consumer_disabled': '该发布者消费者已停用。',
  'identity.publisher_consumer_not_found': '没有找到该发布者消费者。',
  'identity.publisher_idempotency_conflict': '相同请求编号已用于另一项操作，请刷新状态后重新提交。',
  'identity.publisher_key_transition_invalid': '发布者密钥变更无效，请检查密钥状态。',
  'identity.publisher_root_untrusted': '发布者信任根不受信任。',
  'identity.publisher_rotation_pending': '已有发布者密钥轮换正在等待完成。',
  'identity.publisher_signing_unavailable': '发布者签名服务暂时不可用，请稍后再试。',
  'identity.publisher_trust_manifest_invalid': '发布者信任清单无效，请检查后重新提交。',
  'identity.recovery_code_invalid': '恢复代码无效或已经使用。',
  'identity.reset_throttled': '密码重置请求过于频繁，请稍后再试。',
  'identity.self_admin_action_forbidden': '不能对自己的管理员账户执行此操作；请由另一位管理员处理。',
  'identity.session_not_found': '该登录会话已不存在，请刷新会话列表。',
  'identity.step_up_method_unavailable': '当前账户没有可完成此次验证的方式，请先添加身份验证器或通行密钥。',
  'identity.step_up_proof_invalid': '身份验证凭据无效或与当前操作不匹配，请重新验证。',
  'identity.step_up_proof_replayed': '这次身份验证已经使用，请重新验证后再执行操作。',
  'identity.step_up_request_invalid': '身份验证请求无效，请关闭后重新开始。',
  'identity.step_up_required': '这项安全操作需要重新验证身份。',
  'identity.totp_code_invalid': '验证码无效、已过期或已经使用，请等待新验证码后重试。',
  'identity.totp_enrollment_invalid': '设置过程已过期，请重新开始。',
  'identity.totp_not_found': '该身份验证器已不存在，请刷新后确认当前设置。',
  'identity.unknown_role': '指定的账户角色不存在。',
  'identity.verification_invalid': '链接无效、已过期或已经使用，请重新申请。',
  'identity.verify_throttled': '验证邮件发送过于频繁，请稍后再试。',
  'identity.weak_password': '密码不符合安全要求，请更换后重试。',
} as const satisfies Readonly<Record<string, string>>

function stringParam(params: ProblemParams, key: string): string | undefined {
  const value = params[key]
  return typeof value === 'string' ? value : undefined
}

function traceReference(traceId: string | undefined): string {
  return traceId ? `（错误编号：${traceId}）` : ''
}

function weakPasswordMessage(params: ProblemParams): string {
  switch (stringParam(params, 'reason')) {
    case 'too_short':
      return '密码至少需要 8 个字符。'
    case 'too_long':
      return '密码不能超过 128 个字符。'
    case 'blocklisted':
      return '这个密码过于常见或已出现在泄露名单中，请换一个更独特的密码。'
    case 'context_specific':
      return '密码不能与邮箱、邮箱前缀或昵称相同。'
    default:
      return IDENTITY_ERROR_MESSAGES['identity.weak_password']
  }
}

function invalidProfileMessage(params: ProblemParams): string {
  switch (stringParam(params, 'reason')) {
    case 'display_name_required':
    case 'display name required':
      return '昵称不能为空。'
    case 'file_required':
    case 'no file uploaded':
      return '请选择要上传的图片。'
    case 'image_too_large':
    case 'image too large':
      return '图片过大，请压缩后重新上传。'
    case 'unsupported_image':
    case 'not an image':
      return '所选文件不是支持的图片格式。'
    case 'upload_unreadable':
    case 'cannot read upload':
      return '无法读取所选图片，请重新选择文件。'
    default:
      return IDENTITY_ERROR_MESSAGES['identity.invalid_profile']
  }
}

function retryAtMessage(base: string, params: ProblemParams): string {
  const retryAt = stringParam(params, 'retryAt')
  if (!retryAt) return base
  const parsed = new Date(retryAt)
  if (Number.isNaN(parsed.getTime())) return base
  return `${base.replace(/请?稍后再试。$/, '')}可在 ${parsed.toLocaleString('zh-CN')} 后重试。`
}

function identityMessage(
  failure: RemoteFailure,
  context: IdentityErrorContext,
): string | undefined {
  if (!failure.code.startsWith('identity.')) return undefined
  if (failure.code === 'identity.weak_password') {
    return weakPasswordMessage(failure.params)
  }
  if (
    failure.code === 'identity.invalid_credentials'
    && context === 'password-change'
  ) {
    return '当前密码不正确。'
  }
  if (
    failure.code === 'identity.mfa_transaction_invalid'
    && context === 'admin'
  ) {
    return '额外身份验证已过期，请重新发起操作。'
  }
  if (failure.code === 'identity.invalid_profile') {
    return invalidProfileMessage(failure.params)
  }
  if (
    failure.code === 'identity.account_locked'
    || failure.code === 'identity.reset_throttled'
    || failure.code === 'identity.verify_throttled'
  ) {
    const base = IDENTITY_ERROR_MESSAGES[
      failure.code as
        | 'identity.account_locked'
        | 'identity.reset_throttled'
        | 'identity.verify_throttled'
    ]
    return retryAtMessage(base, failure.params)
  }
  if (failure.code === 'identity.pat_limit_reached') {
    const max = failure.params.max
    if (typeof max === 'number') {
      return `访问令牌数量已达上限（${max} 个），请先移除不再使用的令牌。`
    }
  }
  if (failure.code === 'identity.pat_scope_invalid') {
    if (stringParam(failure.params, 'reason') === 'too_many_scopes') {
      const max = failure.params.max
      return typeof max === 'number'
        ? `最多只能选择 ${max} 个访问令牌权限范围。`
        : '选择的访问令牌权限范围过多。'
    }
    const index = failure.params.index
    if (typeof index === 'number') {
      return `第 ${index + 1} 个访问令牌权限范围无效，请检查后重新提交。`
    }
  }
  if (failure.code === 'identity.publisher_attestation_invalid') {
    return stringParam(failure.params, 'reason') === 'attestation_invalid'
      ? '发布者证明内容无效，请检查签名、有效期和声明后重新提交。'
      : '发布者证明请求参数无效，请检查后重新提交。'
  }
  return IDENTITY_ERROR_MESSAGES[
    failure.code as keyof typeof IDENTITY_ERROR_MESSAGES
  ]
}

export function apiErrorMessage(
  error: unknown,
  options: ErrorMessageOptions = {},
): string {
  const failure = getApiFailure(error)
  const fallback = options.fallback ?? '暂时无法完成此操作。'
  if (!failure) return fallback

  if (failure.kind !== 'remote') {
    switch (failure.kind) {
      case 'network':
        return '无法连接服务器，请检查网络后重试。'
      case 'timeout':
        return '请求超时，尚未确认操作是否完成；请先刷新状态。'
      case 'aborted':
        return '操作已取消。'
      case 'protocol':
        return `服务器响应异常，请稍后再试。${traceReference(failure.traceId)}`
    }
  }

  const knownIdentityMessage = identityMessage(
    failure,
    options.context ?? 'generic',
  )
  if (knownIdentityMessage) return knownIdentityMessage

  switch (failure.code) {
    case 'common.unauthorized':
      return '登录状态已失效，请重新登录。'
    case 'common.forbidden':
      return '你没有执行此操作的权限。'
    case 'common.not_found':
      return '目标内容已不存在，请刷新后确认。'
    case 'common.rate_limited':
      return '操作过于频繁，请稍后再试。'
    case 'common.validation_failed':
      return '提交内容不符合要求，请检查后重新提交。'
  }

  if (failure.status === 401) return '登录状态已失效，请重新登录。'
  if (failure.status === 403) return '你没有执行此操作的权限。'
  if (failure.status === 404) return options.fallback ?? '目标内容已不存在，请刷新后确认。'
  if (failure.status === 409 || failure.status === 412 || failure.status === 428) {
    return options.fallback ?? '当前状态已发生变化，请刷新后重新操作。'
  }
  if (failure.status === 429) return '操作过于频繁，请稍后再试。'
  if (failure.status >= 500) {
    return `服务暂时不可用，请稍后再试。${traceReference(failure.traceId)}`
  }
  return `${fallback}${traceReference(failure.traceId)}`
}

export function identityErrorMessage(
  error: unknown,
  options: ErrorMessageOptions = {},
): string {
  return apiErrorMessage(error, options)
}

export function pageErrorMessage(error: unknown): string {
  if (getApiFailure(error)) {
    return apiErrorMessage(error, { fallback: '页面加载失败，请稍后重试。' })
  }
  const status = (error as { statusCode?: unknown })?.statusCode
  if (status === 401) return '登录状态已失效，请重新登录。'
  if (status === 403) return '你没有访问此页面的权限。'
  if (status === 404) return '没有找到这个页面。'
  if (typeof status === 'number' && status >= 500) {
    return '服务暂时不可用，请稍后重试。'
  }
  return '页面加载失败，请稍后重试。'
}

export type OAuthRedirectAction = 'login' | 'register' | 'bind'

export function oauthRedirectErrorMessage(
  error: unknown,
  action: OAuthRedirectAction,
): string {
  const code = Array.isArray(error) ? error[0] : error
  if (typeof code !== 'string' || !code) return ''
  if (action === 'bind') {
    return code === 'oauth_bind'
      ? 'Google 账户绑定失败；该账户可能已绑定到其他用户，请刷新后确认。'
      : 'Google 账户绑定失败，请重新开始。'
  }

  const fallback = action === 'register' ? '邮箱注册' : '邮箱登录'
  switch (code) {
    case 'oauth_unavailable':
      return `Google 登录在当前环境未配置，请使用${fallback}。`
    case 'oauth_state':
      return '第三方登录请求已失效，请重新开始。'
    case 'oauth_denied':
      return `你已取消 Google 授权；可以重新尝试或使用${fallback}。`
    case 'oauth_exchange':
    case 'oauth_userinfo':
      return `暂时无法从 Google 获取账户信息，请重试或使用${fallback}。`
    case 'oauth_login':
      return `Google 登录未完成，请重试或使用${fallback}。`
    default:
      return `第三方登录失败，请重试或使用${fallback}。`
  }
}
