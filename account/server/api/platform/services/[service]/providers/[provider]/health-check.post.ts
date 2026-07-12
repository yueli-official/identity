import { platformServiceKeys, probePlatformProvider, requirePlatformAdmin } from '../../../../../../utils/platform-status'
import type { PlatformServiceKey } from '../../../../../../../shared/types/platform'

export default defineEventHandler(async (event) => {
  await requirePlatformAdmin(event)
  const service = getRouterParam(event, 'service') as PlatformServiceKey
  const provider = getRouterParam(event, 'provider') || ''
  if (!platformServiceKeys.includes(service)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown platform service' })
  }
  return { provider: await probePlatformProvider(event, service, provider) }
})
