import * as z from 'zod'

export const PASSWORD_MIN_LENGTH = 15
export const PASSWORD_MAX_LENGTH = 128
export const PASSWORD_HINT = '至少 15 个字符，建议使用密码管理器生成或保存'

export function passwordLength(value: string): number {
  return [...value.normalize('NFC')].length
}

export function passwordMeetsLengthPolicy(value: string): boolean {
  const length = passwordLength(value)
  return length >= PASSWORD_MIN_LENGTH && length <= PASSWORD_MAX_LENGTH
}

export function newPasswordSchema(label = '密码') {
  return z.string().superRefine((value, context) => {
    const length = passwordLength(value)
    if (length < PASSWORD_MIN_LENGTH) {
      context.addIssue({
        code: 'custom',
        message: `${label}至少 ${PASSWORD_MIN_LENGTH} 个字符`,
      })
    } else if (length > PASSWORD_MAX_LENGTH) {
      context.addIssue({
        code: 'custom',
        message: `${label}最多 ${PASSWORD_MAX_LENGTH} 个字符`,
      })
    }
  })
}
