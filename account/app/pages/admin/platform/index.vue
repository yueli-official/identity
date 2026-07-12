<script setup lang="ts">
import { ManageHeader } from '@platform/manage/components'
import { rel } from '@platform/ui/date'
import type { CapabilityGapReason, PlatformStatusResponse } from '#shared/types/platform'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '平台状态 · 控制台' })

const { data, error, status, refresh } = await useFetch<PlatformStatusResponse>('/api/platform/services', {
  key: 'platform-service-status',
})

const refreshing = ref(false)
const services = computed(() => data.value?.services ?? [])
const issues = computed(() => services.value.flatMap(service =>
  (service.manifest?.capabilities ?? [])
    .filter(capability => capability.support === 'supported' && capability.enablement === 'enabled' && !capability.effective)
    .map(capability => ({ service: service.key, capability })),
))
const gapLabels: Record<CapabilityGapReason, string> = {
  service_unavailable: '服务不可达',
  capability_missing: '能力未声明',
  unsupported: '当前不支持',
  version_incompatible: '版本不兼容',
  configuration_incomplete: '配置不完整',
  disabled: '未启用',
  unhealthy: '运行异常',
}

async function refreshStatus() {
  refreshing.value = true
  try { await refresh() } finally { refreshing.value = false }
}
</script>

<template>
  <div class="space-y-8">
    <ManageHeader title="平台状态">
      <template #subtitle>
        集中查看基础服务、运行时能力和 Provider 状态；配置修改仍由各领域管理页负责。
      </template>
      <template #actions>
        <UButton
          label="刷新状态"
          icon="i-tabler-refresh"
          color="neutral"
          variant="soft"
          :loading="refreshing"
          @click="refreshStatus"
        />
      </template>
    </ManageHeader>

    <UAlert
      v-if="error"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-triangle"
      title="平台状态暂时无法加载"
      description="管理员会话或 Identity BFF 不可用，请稍后重试。"
    />

    <template v-if="status === 'pending' && !data">
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <USkeleton v-for="item in 4" :key="item" class="h-24 rounded-xl" />
      </div>
      <div class="grid gap-4 lg:grid-cols-2">
        <USkeleton v-for="item in 4" :key="item" class="h-72 rounded-xl" />
      </div>
    </template>

    <template v-else-if="data">
      <section aria-labelledby="platform-summary-title">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h2 id="platform-summary-title" class="font-display text-sm font-semibold text-highlighted">运行摘要</h2>
          <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
            <UBadge color="neutral" variant="soft" :label="data.environment" />
            <span class="font-mono">{{ data.catalogFingerprint.slice(0, 12) }}</span>
            <ClientOnly>
              <span>更新于 {{ rel(data.observedAt) }}</span>
              <template #fallback><span>刚刚更新</span></template>
            </ClientOnly>
          </div>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <UCard>
            <p class="text-xs text-muted">基础服务</p>
            <p class="mt-2 text-2xl font-semibold text-highlighted">{{ data.summary.available }} / {{ data.summary.total }}</p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">有效能力</p>
            <p class="mt-2 text-2xl font-semibold text-success">{{ data.summary.effectiveCapabilities }}</p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">启用能力异常</p>
            <p class="mt-2 text-2xl font-semibold" :class="data.summary.capabilityIssues ? 'text-warning' : 'text-success'">
              {{ data.summary.capabilityIssues }}
            </p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">不可达服务</p>
            <p class="mt-2 text-2xl font-semibold" :class="data.summary.available === data.summary.total ? 'text-success' : 'text-error'">
              {{ data.summary.total - data.summary.available }}
            </p>
          </UCard>
        </div>
      </section>

      <section aria-labelledby="platform-services-title">
        <h2 id="platform-services-title" class="font-display mb-3 text-sm font-semibold text-highlighted">基础服务</h2>
        <div class="grid gap-4 lg:grid-cols-2">
          <PlatformServiceCard v-for="service in services" :key="service.key" :service />
        </div>
      </section>

      <section aria-labelledby="application-requirements-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="application-requirements-title" class="font-display text-sm font-semibold text-highlighted">应用能力契约</h2>
          <UBadge :color="data.summary.applicationGaps ? 'warning' : 'success'" variant="soft" :label="`${data.summary.applicationGaps} 项缺口`" />
        </div>
        <div v-if="data.applications.length" class="grid gap-4 lg:grid-cols-2">
          <UCard v-for="application in data.applications" :key="application.site" :ui="{ body: 'space-y-4' }">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate text-sm font-semibold text-highlighted">{{ application.brand || application.site }}</h3>
                <p class="mt-1 font-mono text-xs text-muted">{{ application.site }} · {{ application.productType }}</p>
              </div>
              <UBadge :color="application.satisfied ? 'success' : 'warning'" variant="soft" :label="application.satisfied ? '契约满足' : '存在缺口'" />
            </div>
            <div class="space-y-2">
              <div v-for="requirement in application.requirements" :key="requirement.key" class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-elevated px-3 py-2">
                <div class="min-w-0">
                  <p class="break-all font-mono text-xs font-medium text-highlighted">{{ requirement.key }}</p>
                  <p class="mt-0.5 text-xs text-muted">要求 {{ requirement.constraint }}<template v-if="requirement.actualVersion"> · 当前 {{ requirement.actualVersion }}</template></p>
                </div>
                <UBadge
                  :color="requirement.satisfied ? 'success' : 'error'"
                  variant="subtle"
                  :icon="requirement.satisfied ? 'i-tabler-check' : 'i-tabler-alert-circle'"
                  :label="requirement.satisfied ? '满足' : gapLabels[requirement.reason!]"
                />
              </div>
            </div>
          </UCard>
        </div>
        <UAlert v-else color="neutral" variant="subtle" icon="i-tabler-list-check" title="Catalog 尚未声明应用能力契约" />
      </section>

      <section aria-labelledby="platform-issues-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="platform-issues-title" class="font-display text-sm font-semibold text-highlighted">启用能力异常</h2>
          <UBadge :color="issues.length ? 'warning' : 'success'" variant="soft" :label="`${issues.length} 项`" />
        </div>
        <UCard v-if="issues.length" :ui="{ body: 'divide-y divide-default p-0 sm:p-0' }">
          <NuxtLink
            v-for="issue in issues"
            :key="`${issue.service}:${issue.capability.key}`"
            :to="`/admin/platform/${issue.service}`"
            class="flex min-h-14 items-center justify-between gap-4 px-4 py-3 transition-colors duration-200 hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-highlighted">{{ issue.capability.key }}</p>
              <p class="truncate text-xs text-muted">{{ issue.service }} · {{ issue.capability.configuration }} · {{ issue.capability.health }}</p>
            </div>
            <UIcon name="i-tabler-chevron-right" class="size-4 shrink-0 text-muted" />
          </NuxtLink>
        </UCard>
        <UAlert v-else color="success" variant="subtle" icon="i-tabler-circle-check" title="所有已启用且受支持的能力均有效" />
      </section>
    </template>
  </div>
</template>
