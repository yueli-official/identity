export function expirySeconds(expiresAt: string, now = Date.now()): number {
  const expires = Date.parse(expiresAt)
  if (!Number.isFinite(expires)) return 0
  return Math.max(0, Math.ceil((expires - now) / 1000))
}

export function formatExpiryCountdown(seconds: number): string {
  const minutes = Math.floor(seconds / 60)
  const remainder = seconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(remainder).padStart(2, '0')}`
}

export function useExpiryCountdown(expiresAt: MaybeRefOrGetter<string>) {
  const now = ref(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => {
    timer = setInterval(() => { now.value = Date.now() }, 1000)
  })
  onScopeDispose(() => {
    if (timer) clearInterval(timer)
  })

  const seconds = computed(() => expirySeconds(toValue(expiresAt), now.value))
  const expired = computed(() => !!toValue(expiresAt) && seconds.value === 0)
  const label = computed(() => formatExpiryCountdown(seconds.value))
  return { seconds, expired, label }
}
