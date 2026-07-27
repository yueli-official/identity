import { aggregatePlatformServices, evaluateCapabilityRequirements, fetchPlatformService, readCapabilityRequirements, requirePlatformAdmin } from '../../utils/platform-status'

export default defineEventHandler(async (event) => {
  await requirePlatformAdmin(event)
  const config = useRuntimeConfig(event)
  const observedAt = new Date().toISOString()
  const services = await aggregatePlatformServices(key => fetchPlatformService(event, key))
  const applications = evaluateCapabilityRequirements(await readCapabilityRequirements(event), services)
  return {
    observedAt,
    environment: config.platformEnvironment,
    catalogFingerprint: config.platformCatalogFingerprint,
    summary: {
      total: services.length,
      available: services.filter(service => service.status === 'available').length,
      effectiveCapabilities: services.flatMap(service => service.manifest?.capabilities ?? []).filter(item => item.effective).length,
      capabilityIssues: services.flatMap(service => service.manifest?.capabilities ?? [])
        .filter(item => item.support === 'supported' && item.enablement === 'enabled' && !item.effective).length,
      applications: applications.length,
      applicationGaps: applications.flatMap(application => application.requirements).filter(requirement => !requirement.satisfied).length,
    },
    services,
    applications,
  }
})
