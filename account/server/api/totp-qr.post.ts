import QRCode from 'qrcode'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ value?: unknown }>(event)
  const value = typeof body?.value === 'string' ? body.value : ''
  if (!value.startsWith('otpauth://totp/') || value.length > 2048) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid TOTP enrollment URI' })
  }
  setResponseHeader(event, 'Cache-Control', 'no-store')
  return {
    dataUrl: await QRCode.toDataURL(value, {
      width: 208,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#111827', light: '#ffffff' },
    }),
  }
})
