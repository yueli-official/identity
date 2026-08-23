<script setup lang="ts">
import { PageHeader } from '@yueli/ui/admin'
import { rel } from '~/utils/date'
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
const unavailableServices = computed(() =>
  data.value ? data.value.summary.total - data.value.summary.available : 0,
)
const attentionCount = computed(() =>
  unavailableServices.value
  + (data.value?.summary.capabilityIssues ?? 0)
  + (data.value?.summary.applicationGaps ?? 0),
)
const gapLabels: Record<CapabilityGapReason, string> = {
  service_unavailable: '服务不可达',
  capability_missing: '缺少所需功能',
  unsupported: '当前不支持',
  version_incompatible: '版本不兼容',
  configuration_incomplete: '配置不完整',
  disabled: '未启用',
  unhealthy: '运行异常',
}
const configurationLabels: Record<string, string> = {
  complete: '配置完整', partial: '部分配置缺失', missing: '未配置',
}
const healthLabels: Record<string, string> = {
  healthy: '运行正常', degraded: '运行受限', unhealthy: '运行异常', unknown: '尚未检查',
}

async function refreshStatus() {
  refreshing.value = true
  try { await refresh() } finally { refreshing.value = false }
}
</script>

<template>
  <div class="space-y-5">
    <PageHeader title="平台状态" icon="i-tabler-activity-heartbeat">
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
    </PageHeader>

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
          <h2 id="platform-summary-title" class="font-display text-sm font-semibold text-highlighted">整体状态</h2>
          <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
            <UBadge color="neutral" variant="soft" :label="data.environment" />
            <ClientOnly>
              <span>更新于 {{ rel(data.observedAt) }}</span>
              <template #fallback><span>刚刚更新</span></template>
            </ClientOnly>
          </div>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <UCard>
            <p class="text-xs text-muted">已连接服务</p>
            <p class="mt-2 text-2xl font-semibold text-highlighted">{{ data.summary.available }} / {{ data.summary.total }}</p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">可用功能</p>
            <p class="mt-2 text-2xl font-semibold text-success">{{ data.summary.effectiveCapabilities }}</p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">功能异常</p>
            <p class="mt-2 text-2xl font-semibold" :class="data.summary.capabilityIssues ? 'text-warning' : 'text-success'">
              {{ data.summary.capabilityIssues }}
            </p>
          </UCard>
          <UCard>
            <p class="text-xs text-muted">未连接服务</p>
            <p class="mt-2 text-2xl font-semibold" :class="data.summary.available === data.summary.total ? 'text-success' : 'text-error'">
              {{ data.summary.total - data.summary.available }}
            </p>
          </UCard>
        </div>
      </section>

      <UAlert
        v-if="attentionCount"
        color="warning"
        variant="subtle"
        icon="i-tabler-alert-triangle"
        :title="`${attentionCount} 项状态需要处理`"
        :description="`${unavailableServices} 个服务未连接，${data.summary.capabilityIssues} 项功能不可用，${data.summary.applicationGaps} 项应用依赖未满足。`"
      />
      <UAlert
        v-else
        color="success"
        variant="subtle"
        icon="i-tabler-circle-check"
        title="平台服务运行正常"
        description="所有基础服务均已连接，已启用功能和应用依赖都处于正常状态。"
      />

      <section aria-labelledby="platform-services-title">
        <h2 id="platform-services-title" class="font-display mb-3 text-sm font-semibold text-highlighted">服务状态</h2>
        <div class="grid gap-4 lg:grid-cols-2">
          <PlatformServiceCard v-for="service in services" :key="service.key" :service />
        </div>
      </section>

      <section aria-labelledby="application-requirements-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="application-requirements-title" class="font-display text-sm font-semibold text-highlighted">应用依赖</h2>
          <UBadge :color="data.summary.applicationGaps ? 'warning' : 'success'" variant="soft" :label="`${data.summary.applicationGaps} 项未满足`" />
        </div>
        <div v-if="data.applications.length" class="grid gap-4 lg:grid-cols-2">
          <UCard v-for="application in data.applications" :key="application.site" :ui="{ body: 'space-y-4' }">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate text-sm font-semibold text-highlighted">{{ application.brand || application.site }}</h3>
                <p class="mt-1 font-mono text-xs text-muted">{{ application.site }} · {{ application.productType }}</p>
              </div>
              <UBadge :color="application.satisfied ? 'success' : 'warning'" variant="soft" :label="application.satisfied ? '依赖正常' : '依赖缺失'" />
            </div>
            <div class="space-y-2">
              <div v-for="requirement in application.requirements" :key="requirement.key" class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-elevated px-3 py-2">
                <div class="min-w-0">
                  <p class="break-all font-mono text-xs font-medium text-highlighted">{{ requirement.key }}</p>
                  <p class="mt-0.5 text-xs text-muted">需要版本 {{ requirement.constraint }}<template v-if="requirement.actualVersion"> · 当前提供 {{ requirement.actualVersion }}</template></p>
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
        <UAlert
          v-else
          color="neutral"
          variant="subtle"
          icon="i-tabler-list-check"
          title="尚未配置应用依赖"
          description="当前没有应用声明它依赖哪些基础功能，不影响服务本身运行。"
        />
      </section>

      <section aria-labelledby="platform-issues-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="platform-issues-title" class="font-display text-sm font-semibold text-highlighted">功能异常</h2>
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
              <p class="truncate text-xs text-muted">
                {{ issue.service }} · {{ configurationLabels[issue.capability.configuration] || issue.capability.configuration }} ·
                {{ healthLabels[issue.capability.health] || issue.capability.health }}
              </p>
            </div>
            <UIcon name="i-tabler-chevron-right" class="size-4 shrink-0 text-muted" />
          </NuxtLink>
        </UCard>
        <UAlert v-else color="success" variant="subtle" icon="i-tabler-circle-check" title="所有已启用功能均可用" />
      </section>
    </template>
  </div>
</template>
