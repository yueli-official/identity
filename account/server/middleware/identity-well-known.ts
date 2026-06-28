export default defineEventHandler((event) => {
  if (!event.path.startsWith('/.well-known/')) return

  const base = useRuntimeConfig().apiBase.replace(/\/$/, '')
  return proxyRequest(event, base + event.path)
})
