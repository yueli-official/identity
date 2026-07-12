<script setup lang="ts">
import { rel } from '@platform/ui/date'
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
        <p class="text-xs text-muted">有效能力</p>
        <p class="mt-1 font-semibold text-highlighted">{{ effectiveCount }} / {{ capabilityCount }}</p>
      </div>
      <div class="rounded-lg bg-elevated p-3">
        <p class="text-xs text-muted">Provider</p>
        <p class="mt-1 font-semibold text-highlighted">{{ service.manifest.providers.length }}</p>
      </div>
    </div>
    <UAlert
      v-else
      color="error"
      variant="subtle"
      icon="i-tabler-plug-connected-x"
      title="无法读取服务状态"
      :description="service.error?.message || '服务未返回有效状态'"
    />

    <div class="mt-auto flex items-center justify-between gap-3 border-t border-default pt-4 text-xs text-muted">
      <span>{{ service.latencyMs }} ms</span>
      <ClientOnly>
        <span v-if="service.manifest">快照 {{ rel(service.manifest.generatedAt) }}</span>
        <template #fallback><span>快照时间</span></template>
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
