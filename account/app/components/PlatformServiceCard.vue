<script setup lang="ts">
import { rel } from '~/utils/date'
import type { PlatformServiceResult } from '#shared/types/platform'

const { service } = defineProps<{ service: PlatformServiceResult }>()

const serviceMeta = {
  identity: { label: 'Identity', description: '认证、用户与令牌', icon: 'i-tabler-fingerprint' },
  asset: { label: 'Asset', description: '资源与对象存储', icon: 'i-tabler-photo-cog' },
  commerce: { label: 'Commerce', description: '支付与交易能力', icon: 'i-tabler-credit-card-pay' },
  notification: { label: 'Notification', description: '通知渠道与场景', icon: 'i-tabler-bell-cog' },
} as const

const meta = computed(() => serviceMeta[service.key])
const capabilityCount = computed(() => service.manifest?.capabilities.length ?? 0)
const effectiveCount = computed(() => service.manifest?.capabilities.filter(item => item.effective).length ?? 0)
const issueCount = computed(() => service.manifest?.capabilities
  .filter(item => item.support === 'supported' && item.enablement === 'enabled' && !item.effective).length ?? 0)
const tone = computed(() => {
  if (service.status !== 'available') return { color: 'error' as const, label: '不可用', icon: 'i-tabler-alert-triangle' }
  if (issueCount.value > 0) return { color: 'warning' as const, label: '需关注', icon: 'i-tabler-alert-circle' }
  return { color: 'success' as const, label: '正常', icon: 'i-tabler-circle-check' }
})
const serviceErrorMessage = computed(() => {
  const message = service.error?.message || ''
  if (/deadline|timeout/i.test(message)) return '服务在规定时间内没有响应'
  if (/access was denied|forbidden/i.test(message)) return '当前账号无权读取该服务状态'
  return '服务没有返回可识别的运行状态'
})
</script>

<template>
  <UCard class="h-full" :ui="{ body: 'flex h-full flex-col gap-5' }">
    <div class="flex items-start justify-between gap-4">
      <div class="flex min-w-0 items-center gap-3">
        <span class="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <UIcon :name="meta.icon" class="size-5" />
        </span>
        <div class="min-w-0">
          <h2 class="font-display truncate text-base font-semibold text-highlighted">{{ meta.label }}</h2>
          <p class="truncate text-sm text-muted">{{ meta.description }}</p>
        </div>
      </div>
      <UBadge :color="tone.color" variant="soft" :icon="tone.icon" :label="tone.label" />
    </div>

    <div v-if="service.manifest" class="grid grid-cols-2 gap-3 text-sm">
      <div class="rounded-lg bg-elevated p-3">
        <p class="text-xs text-muted">可用功能</p>
        <p class="mt-1 font-semibold text-highlighted">{{ effectiveCount }} / {{ capabilityCount }}</p>
      </div>
      <div class="rounded-lg bg-elevated p-3">
        <p class="text-xs text-muted">服务来源</p>
        <p class="mt-1 font-semibold text-highlighted">{{ service.manifest.providers.length }}</p>
      </div>
    </div>
    <UAlert
      v-else
      color="error"
      variant="subtle"
      icon="i-tabler-plug-connected-x"
      title="无法读取服务状态"
      :description="serviceErrorMessage"
    />

    <div class="mt-auto flex items-center justify-between gap-3 border-t border-default pt-4 text-xs text-muted">
      <span>{{ service.latencyMs }} ms</span>
      <ClientOnly>
        <span v-if="service.manifest">更新 {{ rel(service.manifest.generatedAt) }}</span>
        <template #fallback><span>更新时间</span></template>
      </ClientOnly>
      <UButton
        :to="`/admin/platform/${service.key}`"
        label="查看详情"
        icon="i-tabler-arrow-right"
        trailing
        color="neutral"
        variant="ghost"
        size="xs"
      />
    </div>
  </UCard>
</template>
