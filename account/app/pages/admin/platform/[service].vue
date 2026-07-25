<script setup lang="ts">
import { PageHeader } from '@yueli/ui/dashboard/pattern'
import { rel } from '@platform/ui/date'
import type { PlatformServiceKey, PlatformStatusResponse, RuntimeHealth } from '#shared/types/platform'

definePageMeta({ layout: 'admin', middleware: 'admin' })

const route = useRoute()
const serviceKey = computed(() => String(route.params.service) as PlatformServiceKey)
const validServices: PlatformServiceKey[] = ['identity', 'asset', 'commerce', 'notification']
if (!validServices.includes(serviceKey.value)) {
  throw createError({ statusCode: 404, statusMessage: 'Unknown platform service' })
}

const labels: Record<PlatformServiceKey, string> = {
  identity: 'Identity', asset: 'Asset', commerce: 'Commerce', notification: 'Notification',
}
const manageLinks: Partial<Record<PlatformServiceKey, string>> = {
  identity: '/admin/users', asset: '/admin/assets',
}
useSeoMeta({ title: () => `${labels[serviceKey.value]} · 平台状态` })

const { data, error, status, refresh } = await useFetch<PlatformStatusResponse>('/api/platform/services', {
  key: 'platform-service-status',
})
const service = computed(() => data.value?.services.find(item => item.key === serviceKey.value))
const manifest = computed(() => service.value?.manifest)
const domainLinks = computed(() => {
  if (!manifest.value) return []
  const links = [
    ...manifest.value.links.map(link => ({ ...link, source: 'service' })),
    ...manifest.value.capabilities.flatMap(item => item.links.map(link => ({ ...link, source: item.key }))),
    ...manifest.value.providers.flatMap(item => item.links.map(link => ({ ...link, source: item.key }))),
  ]
  return links.filter((link, index) => links.findIndex(candidate => candidate.rel === link.rel && candidate.href === link.href) === index)
})
const refreshing = ref(false)
const probingProvider = ref('')
const probeError = ref('')
const probeMessage = ref('')

const healthTone: Record<RuntimeHealth, { color: 'neutral' | 'success' | 'warning' | 'error', label: string }> = {
  unknown: { color: 'neutral', label: '未探测' },
  healthy: { color: 'success', label: '健康' },
  degraded: { color: 'warning', label: '降级' },
  unhealthy: { color: 'error', label: '异常' },
}

async function refreshStatus() {
  refreshing.value = true
  try { await refresh() } finally { refreshing.value = false }
}

async function probeProvider(provider: string) {
  probingProvider.value = provider
  probeError.value = ''
  probeMessage.value = ''
  try {
    await $fetch(`/api/platform/services/${serviceKey.value}/providers/${encodeURIComponent(provider)}/health-check`, { method: 'POST' })
    await refresh()
    probeMessage.value = `${provider} 探测完成，状态已刷新`
  } catch (error) {
    probeError.value = error instanceof Error ? error.message : 'Provider 探测失败'
  } finally {
    probingProvider.value = ''
  }
}
</script>

<template>
  <div class="space-y-8">
    <PageHeader :title="labels[serviceKey]">
      <template #subtitle>
        运行时能力与 Provider 快照；这里只读展示状态，不复制领域配置真值。
      </template>
      <template #actions>
        <UButton to="/admin/platform" icon="i-tabler-arrow-left" label="返回平台" color="neutral" variant="ghost" />
        <UButton v-if="manageLinks[serviceKey]" :to="manageLinks[serviceKey]" icon="i-tabler-settings" label="领域管理" color="neutral" variant="soft" />
        <UButton icon="i-tabler-refresh" label="刷新" color="neutral" variant="soft" :loading="refreshing" @click="refreshStatus" />
      </template>
    </PageHeader>

    <UAlert v-if="error" color="error" variant="subtle" icon="i-tabler-alert-triangle" title="状态加载失败" description="请检查管理员会话与 Identity BFF。" />
    <div v-else-if="status === 'pending' && !data" class="space-y-4">
      <USkeleton class="h-36 rounded-xl" />
      <USkeleton class="h-72 rounded-xl" />
    </div>
    <UAlert
      v-else-if="!manifest"
      color="error"
      variant="subtle"
      icon="i-tabler-plug-connected-x"
      title="服务不可用"
      :description="service?.error?.message || '没有可用的 Capability Manifest。'"
    />

    <template v-else>
      <UCard>
        <div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <p class="text-xs text-muted">版本</p>
            <p class="mt-1 font-mono text-sm font-medium text-highlighted">{{ manifest.service.version }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">Build SHA</p>
            <p class="mt-1 truncate font-mono text-sm font-medium text-highlighted">{{ manifest.service.buildSha }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">Deployment</p>
            <p class="mt-1 truncate font-mono text-sm font-medium text-highlighted">{{ manifest.service.deployment }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">快照</p>
            <ClientOnly>
              <p class="mt-1 text-sm font-medium text-highlighted">{{ rel(manifest.generatedAt) }}</p>
              <template #fallback><p class="mt-1 text-sm text-muted">正在同步时间</p></template>
            </ClientOnly>
          </div>
        </div>
      </UCard>

      <section aria-labelledby="capabilities-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="capabilities-title" class="font-display text-sm font-semibold text-highlighted">运行能力</h2>
          <UBadge color="neutral" variant="soft" :label="`${manifest.capabilities.length} 项`" />
        </div>
        <div class="space-y-3">
          <UCard v-for="capability in manifest.capabilities" :key="capability.key" :ui="{ body: 'space-y-4' }">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-mono text-sm font-semibold text-highlighted">{{ capability.key }}</h3>
                  <UBadge :color="capability.effective ? 'success' : 'warning'" variant="soft" :label="capability.effective ? '有效' : '不可用'" />
                </div>
                <p class="mt-1 text-xs text-muted">契约 {{ capability.contractVersion }} · {{ capability.operations.join(' · ') }}</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <UBadge color="neutral" variant="outline" :label="`配置 ${capability.configuration}`" />
                <UBadge color="neutral" variant="outline" :label="`启用 ${capability.enablement}`" />
                <UBadge :color="healthTone[capability.health].color" variant="soft" :label="healthTone[capability.health].label" />
              </div>
            </div>
            <div v-if="capability.requiredConfig.length" class="flex flex-wrap gap-2">
              <UBadge
                v-for="field in capability.requiredConfig"
                :key="field.key"
                :color="field.state === 'present' ? 'success' : 'error'"
                variant="subtle"
                :icon="field.secret ? 'i-tabler-lock' : 'i-tabler-adjustments'"
                :label="`${field.key}: ${field.state}`"
              />
            </div>
          </UCard>
        </div>
      </section>

      <section aria-labelledby="providers-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="providers-title" class="font-display text-sm font-semibold text-highlighted">Provider 实例</h2>
          <UBadge color="neutral" variant="soft" :label="`${manifest.providers.length} 项`" />
        </div>
        <div aria-live="polite" aria-atomic="true">
          <UAlert v-if="probeError" class="mb-4" color="error" variant="subtle" icon="i-tabler-alert-triangle" title="Provider 探测失败" :description="probeError" />
          <UAlert v-else-if="probeMessage" class="mb-4" color="success" variant="subtle" icon="i-tabler-circle-check" title="Provider 探测完成" :description="probeMessage" />
        </div>
        <div v-if="manifest.providers.length" class="grid gap-4 lg:grid-cols-2">
          <UCard v-for="provider in manifest.providers" :key="provider.key" :ui="{ body: 'space-y-4' }">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-mono text-sm font-semibold text-highlighted">{{ provider.key }}</h3>
                <p class="mt-1 text-xs text-muted">{{ provider.adapter }}<template v-if="provider.mode"> · {{ provider.mode }}</template></p>
              </div>
              <UBadge :color="provider.effective ? 'success' : 'warning'" variant="soft" :label="provider.effective ? '有效' : '不可用'" />
            </div>
            <div class="grid grid-cols-2 gap-3 text-xs">
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">注册</span><p class="mt-1 font-medium text-highlighted">{{ provider.registered ? '是' : '否' }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">配置</span><p class="mt-1 font-medium text-highlighted">{{ provider.configuration }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">启用</span><p class="mt-1 font-medium text-highlighted">{{ provider.enablement }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">健康</span><p class="mt-1 font-medium text-highlighted">{{ healthTone[provider.health].label }}</p></div>
            </div>
            <div v-if="provider.requiredConfig.length" class="flex flex-wrap gap-2">
              <UBadge
                v-for="field in provider.requiredConfig"
                :key="field.key"
                :color="field.state === 'present' ? 'success' : 'error'"
                variant="subtle"
                :icon="field.secret ? 'i-tabler-lock' : 'i-tabler-adjustments'"
                :label="`${field.key}: ${field.state}`"
              />
            </div>
            <div class="flex flex-wrap gap-1.5">
              <UBadge v-for="operation in provider.operations" :key="operation" color="neutral" variant="outline" :label="operation" />
            </div>
            <UButton
              icon="i-tabler-heart-rate-monitor"
              label="主动探测"
              color="neutral"
              variant="soft"
              size="sm"
              :loading="probingProvider === provider.key"
              :disabled="Boolean(probingProvider)"
              @click="probeProvider(provider.key)"
            />
          </UCard>
        </div>
        <UAlert v-else color="neutral" variant="subtle" icon="i-tabler-package-off" title="该服务没有 Provider Instance" />
      </section>

      <section id="management-links" aria-labelledby="management-links-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="management-links-title" class="font-display text-sm font-semibold text-highlighted">领域管理与服务入口</h2>
          <UBadge color="neutral" variant="soft" :label="`${domainLinks.length} 项服务声明`" />
        </div>
        <UCard :ui="{ body: 'space-y-4' }">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="text-sm font-medium text-highlighted">配置真值归所属服务管理</p>
              <p class="mt-1 text-xs text-muted">控制中心只展示 Manifest 声明，不复制 Provider 配置。</p>
            </div>
            <UButton v-if="manageLinks[serviceKey]" :to="manageLinks[serviceKey]" icon="i-tabler-settings" label="打开管理界面" color="neutral" variant="soft" />
            <UBadge v-else color="warning" variant="subtle" label="尚无统一管理界面" />
          </div>
          <div v-if="domainLinks.length" class="divide-y divide-default rounded-lg border border-default">
            <div v-for="link in domainLinks" :key="`${link.rel}:${link.href}`" class="grid gap-1 px-3 py-2.5 sm:grid-cols-[10rem_1fr_auto] sm:items-center">
              <span class="text-xs font-medium text-muted">{{ link.rel }}</span>
              <code class="break-all text-xs text-highlighted">{{ link.href }}</code>
              <UBadge color="neutral" variant="outline" :label="link.source" />
            </div>
          </div>
        </UCard>
      </section>
    </template>
  </div>
</template>
