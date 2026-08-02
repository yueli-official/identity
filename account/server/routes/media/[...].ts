// Stable browser media URLs stay unversioned while this adapter explicitly
// selects Asset's v1 implementation interface.
export default defineEventHandler((event) => {
  const base = useRuntimeConfig(event).assetBase.replace(/\/$/, '')
  const suffix = event.path.slice('/media'.length)
  return proxyRequest(event, `${base}/api/v1/media${suffix}`, {
    streamRequest: true
  })
})
