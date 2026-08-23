<script setup lang="ts">
import { PageHeader } from '@yueli/ui/admin'
import { rel } from '~/utils/date'
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
const icons: Record<PlatformServiceKey, string> = {
  identity: 'i-tabler-fingerprint',
  asset: 'i-tabler-photo-cog',
  commerce: 'i-tabler-shopping-cart-cog',
  notification: 'i-tabler-bell-cog',
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
const { call } = useApi()

const healthTone: Record<RuntimeHealth, { color: 'neutral' | 'success' | 'warning' | 'error', label: string }> = {
  unknown: { color: 'neutral', label: '尚未检查' },
  healthy: { color: 'success', label: '运行正常' },
  degraded: { color: 'warning', label: '运行受限' },
  unhealthy: { color: 'error', label: '运行异常' },
}
const configurationLabels: Record<string, string> = {
  complete: '配置完整', partial: '部分配置缺失', missing: '未配置',
}
const enablementLabels: Record<string, string> = {
  enabled: '已启用', disabled: '未启用',
}
const fieldStateLabels: Record<string, string> = {
  present: '已配置', missing: '缺失',
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
    await call(`/api/platform/services/${serviceKey.value}/providers/${encodeURIComponent(provider)}/health-check`, { method: 'POST' })
    await refresh()
    probeMessage.value = `${provider} 连接检查完成，状态已刷新`
  } catch (error) {
    probeError.value = apiErrorMessage(error, {
      context: 'admin',
      fallback: '暂时无法检查服务来源。',
    })
  } finally {
    probingProvider.value = ''
  }
}
</script>

<template>
  <div class="space-y-5">
    <PageHeader :title="labels[serviceKey]" :icon="icons[serviceKey]">
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
      description="服务没有返回运行状态，请确认服务已启动且当前账号有读取权限。"
    />

    <template v-else>
      <UCard>
        <div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <p class="text-xs text-muted">版本</p>
            <p class="mt-1 font-mono text-sm font-medium text-highlighted">{{ manifest.service.version }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">构建标识</p>
            <p class="mt-1 truncate font-mono text-sm font-medium text-highlighted">{{ manifest.service.buildSha }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">部署实例</p>
            <p class="mt-1 truncate font-mono text-sm font-medium text-highlighted">{{ manifest.service.deployment }}</p>
          </div>
          <div>
            <p class="text-xs text-muted">状态更新时间</p>
            <ClientOnly>
              <p class="mt-1 text-sm font-medium text-highlighted">{{ rel(manifest.generatedAt) }}</p>
              <template #fallback><p class="mt-1 text-sm text-muted">正在同步时间</p></template>
            </ClientOnly>
          </div>
        </div>
      </UCard>

      <section aria-labelledby="capabilities-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="capabilities-title" class="font-display text-sm font-semibold text-highlighted">可用功能</h2>
          <UBadge color="neutral" variant="soft" :label="`${manifest.capabilities.length} 项`" />
        </div>
        <div class="space-y-3">
          <UCard v-for="capability in manifest.capabilities" :key="capability.key" :ui="{ body: 'space-y-4' }">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-mono text-sm font-semibold text-highlighted">{{ capability.key }}</h3>
                  <UBadge :color="capability.effective ? 'success' : 'warning'" variant="soft" :label="capability.effective ? '可用' : '需处理'" />
                </div>
                <p class="mt-1 text-xs text-muted">接口版本 {{ capability.contractVersion }} · 支持 {{ capability.operations.length }} 项操作</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <UBadge color="neutral" variant="outline" :label="configurationLabels[capability.configuration] || capability.configuration" />
                <UBadge color="neutral" variant="outline" :label="enablementLabels[capability.enablement] || capability.enablement" />
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
                :label="`${field.key}: ${fieldStateLabels[field.state] || field.state}`"
              />
            </div>
          </UCard>
        </div>
      </section>

      <section aria-labelledby="providers-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="providers-title" class="font-display text-sm font-semibold text-highlighted">服务来源</h2>
          <UBadge color="neutral" variant="soft" :label="`${manifest.providers.length} 项`" />
        </div>
        <div aria-live="polite" aria-atomic="true">
          <UAlert v-if="probeError" class="mb-4" color="error" variant="subtle" icon="i-tabler-alert-triangle" title="连接检查失败" :description="probeError" />
          <UAlert v-else-if="probeMessage" class="mb-4" color="success" variant="subtle" icon="i-tabler-circle-check" title="连接检查完成" :description="probeMessage" />
        </div>
        <div v-if="manifest.providers.length" class="grid gap-4 lg:grid-cols-2">
          <UCard v-for="provider in manifest.providers" :key="provider.key" :ui="{ body: 'space-y-4' }">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-mono text-sm font-semibold text-highlighted">{{ provider.key }}</h3>
                <p class="mt-1 text-xs text-muted">{{ provider.adapter }}<template v-if="provider.mode"> · {{ provider.mode }}</template></p>
              </div>
              <UBadge :color="provider.effective ? 'success' : 'warning'" variant="soft" :label="provider.effective ? '可用' : '需处理'" />
            </div>
            <div class="grid grid-cols-2 gap-3 text-xs">
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">已接入</span><p class="mt-1 font-medium text-highlighted">{{ provider.registered ? '是' : '否' }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">配置状态</span><p class="mt-1 font-medium text-highlighted">{{ configurationLabels[provider.configuration] || provider.configuration }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">启用状态</span><p class="mt-1 font-medium text-highlighted">{{ enablementLabels[provider.enablement] || provider.enablement }}</p></div>
              <div class="rounded-lg bg-elevated p-3"><span class="text-muted">运行状态</span><p class="mt-1 font-medium text-highlighted">{{ healthTone[provider.health].label }}</p></div>
            </div>
            <div v-if="provider.requiredConfig.length" class="flex flex-wrap gap-2">
              <UBadge
                v-for="field in provider.requiredConfig"
                :key="field.key"
                :color="field.state === 'present' ? 'success' : 'error'"
                variant="subtle"
                :icon="field.secret ? 'i-tabler-lock' : 'i-tabler-adjustments'"
                :label="`${field.key}: ${fieldStateLabels[field.state] || field.state}`"
              />
            </div>
            <UButton
              icon="i-tabler-heart-rate-monitor"
              label="检查连接"
              color="neutral"
              variant="soft"
              size="sm"
              :loading="probingProvider === provider.key"
              :disabled="Boolean(probingProvider)"
              @click="probeProvider(provider.key)"
            />
          </UCard>
        </div>
        <UAlert v-else color="neutral" variant="subtle" icon="i-tabler-package-off" title="该服务没有可检查的服务来源" />
      </section>

      <section id="management-links" aria-labelledby="management-links-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="management-links-title" class="font-display text-sm font-semibold text-highlighted">管理入口</h2>
          <UBadge color="neutral" variant="soft" :label="`${domainLinks.length} 项接口信息`" />
        </div>
        <UCard :ui="{ body: 'space-y-4' }">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="text-sm font-medium text-highlighted">配置由对应服务维护</p>
              <p class="mt-1 text-xs text-muted">这里汇总运行状态，实际配置仍在对应管理页面修改。</p>
            </div>
            <UButton v-if="manageLinks[serviceKey]" :to="manageLinks[serviceKey]" icon="i-tabler-settings" label="打开管理界面" color="neutral" variant="soft" />
            <UBadge v-else color="warning" variant="subtle" label="尚无统一管理界面" />
          </div>
          <UCollapsible v-if="domainLinks.length">
            <UButton
              color="neutral"
              variant="ghost"
              trailing-icon="i-tabler-chevron-down"
              label="查看接口信息"
            />
            <template #content>
              <div class="mt-3 divide-y divide-default rounded-lg border border-default">
                <div v-for="link in domainLinks" :key="`${link.rel}:${link.href}`" class="grid gap-1 px-3 py-2.5 sm:grid-cols-[10rem_1fr_auto] sm:items-center">
                  <span class="text-xs font-medium text-muted">{{ link.rel }}</span>
                  <code class="break-all text-xs text-highlighted">{{ link.href }}</code>
                  <UBadge color="neutral" variant="outline" :label="link.source" />
                </div>
              </div>
            </template>
          </UCollapsible>
        </UCard>
      </section>
    </template>
  </div>
</template>
