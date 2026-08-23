<script setup lang="ts">
import type {
  AssetConsumerRegistrationState,
  AssetRegistrationApplication,
  AssetRegistrationProfile,
} from '~/types/asset-admin'

defineProps<{ states: AssetConsumerRegistrationState[] }>()

function latestApplication(state: AssetConsumerRegistrationState) {
  return [...state.applications].sort((left, right) => right.revision - left.revision)[0]
}

function displayName(state: AssetConsumerRegistrationState) {
  return state.registration?.effective.displayName || latestApplication(state)?.declaration.displayName || state.consumerKey
}

function namespaceKey(state: AssetConsumerRegistrationState) {
  return state.registration?.namespaceKey || latestApplication(state)?.declaration.namespaceKey || '—'
}

function currentProfiles(state: AssetConsumerRegistrationState) {
  return state.registration?.effective.profiles || []
}

function pendingApplication(state: AssetConsumerRegistrationState) {
  return [...state.applications].reverse().find(application => application.status === 'pending')
}

function status(state: AssetConsumerRegistrationState) {
  if (pendingApplication(state)) return { label: state.registration ? '有待处理变更' : '等待注册', color: 'warning' as const }
  if (state.registration) return { label: '已生效', color: 'success' as const }
  return { label: '未生效', color: 'neutral' as const }
}

function profileKind(profile: AssetRegistrationProfile) {
  return {
    'public-image': '公开图片',
    'private-original': '私有原件',
    'public-file': '公开文件',
    'private-file': '私有文件',
  }[profile.kind]
}

function formatBytes(bytes: number) {
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${Number((bytes / 1024 / 1024).toFixed(1))} MB`
  return `${Number((bytes / 1024 / 1024 / 1024).toFixed(1))} GB`
}

function applicationLabel(application: AssetRegistrationApplication) {
  return {
    accepted: '已接受',
    pending: '待处理',
    rejected: '已拒绝',
  }[application.status]
}

function applicationColor(application: AssetRegistrationApplication) {
  return application.status === 'accepted' ? 'success' : application.status === 'pending' ? 'warning' : 'error'
}
</script>

<template>
  <section aria-label="消费者注册" class="space-y-4">
    <ManageEmpty v-if="!states.length" icon="i-tabler-file-import" text="还没有消费者提交资源声明" />

    <article
      v-for="state in states"
      v-else
      :key="state.consumerKey"
      class="overflow-hidden rounded-xl border border-default bg-default"
      data-asset-registration-card
    >
      <header class="flex flex-col gap-4 p-5 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex min-w-0 gap-3">
          <span class="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
            <UIcon name="i-tabler-plug-connected" class="size-5" />
          </span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="font-semibold text-highlighted">{{ displayName(state) }}</h3>
              <UBadge :label="status(state).label" :color="status(state).color" variant="soft" />
            </div>
            <p class="mt-1 text-sm text-muted">
              {{ state.consumerKey }} · Namespace {{ namespaceKey(state) }}
            </p>
          </div>
        </div>
      </header>

      <div v-if="currentProfiles(state).length" class="border-t border-default px-5">
        <div
          v-for="profile in currentProfiles(state)"
          :key="profile.key"
          class="grid gap-3 border-b border-default py-4 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <p class="font-medium text-highlighted">{{ profile.purpose }}</p>
              <UBadge :label="profile.key" color="neutral" variant="soft" />
              <UBadge :label="profileKind(profile)" color="neutral" variant="outline" />
            </div>
            <p class="mt-1 text-sm text-muted">
              {{ formatBytes(profile.maxBytes) }} · {{ profile.allowedMimes.length }} 种格式 · {{ profile.variants.length }} 个派生规格
            </p>
          </div>
          <div class="text-sm sm:text-right">
            <p class="font-medium text-highlighted">{{ profile.storageBackend || '尚未分配后端' }}</p>
            <p class="mt-1 text-xs text-muted">{{ profile.storageClass }}</p>
          </div>
        </div>
      </div>

      <UCollapsible v-if="state.applications.length" class="border-t border-default">
        <UButton
          color="neutral"
          variant="ghost"
          block
          trailing-icon="i-tabler-chevron-down"
          class="justify-between rounded-none px-5 py-3 text-muted"
          :label="`申请记录 · ${state.applications.length}`"
        />
        <template #content>
          <div class="border-t border-default px-5 py-2">
            <div
              v-for="application in [...state.applications].reverse()"
              :key="application.revision"
              class="flex flex-col gap-2 border-b border-default py-3 last:border-b-0 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium text-highlighted">Revision {{ application.revision }}</span>
                <UBadge :label="applicationLabel(application)" :color="applicationColor(application)" variant="soft" />
                <span v-if="application.findings.length" class="text-xs text-warning">
                  {{ application.findings.length }} 项超出当前约束
                </span>
              </div>
              <UTooltip :text="application.declarationDigest">
                <span class="font-mono text-xs text-muted">{{ application.declarationDigest.slice(0, 12) }}</span>
              </UTooltip>
            </div>
          </div>
        </template>
      </UCollapsible>
    </article>
  </section>
</template>
