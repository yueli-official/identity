<script setup lang="ts">
import { ManageEmpty } from '~/components/manage'
import type { AssetStorageBackend } from '~/types/asset-admin'

defineProps<{ backends: readonly AssetStorageBackend[] }>()
const emit = defineEmits<{ edit: [backend: AssetStorageBackend] }>()

function health(backend: AssetStorageBackend) {
  if (backend.enabled === false) return { label: '停用', color: 'neutral' as const }
  if (!backend.healthy) return { label: '异常', color: 'error' as const }
  return { label: '正常', color: 'success' as const }
}
</script>

<template>
  <section aria-label="存储后端">
    <ManageEmpty v-if="!backends.length" icon="i-tabler-database-off" text="还没有存储后端" />
    <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
      <div v-for="backend in backends" :key="backend.name" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b border-default px-4 py-3 last:border-b-0">
        <div class="min-w-0">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
              <UIcon name="i-tabler-database" class="size-4" />
            </span>
            <div class="min-w-0">
              <h3 class="truncate text-sm font-semibold text-highlighted">{{ backend.name }}</h3>
              <p class="truncate text-xs text-muted">{{ backend.type || 'S3-compatible' }}</p>
            </div>
            <UBadge v-if="backend.isDefault" label="平台默认" color="primary" variant="soft" size="sm" />
            <UBadge :label="health(backend).label" :color="health(backend).color" variant="soft" size="sm" />
          </div>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 pl-11 text-xs text-muted">
            <span>{{ backend.assetCount || 0 }} 素材</span>
            <span>{{ backend.consumerCount || 0 }} 消费者</span>
            <span>{{ backend.profileCount || 0 }} 用途</span>
            <span v-if="backend.error" class="text-error">{{ backend.error }}</span>
          </div>
        </div>
        <UTooltip :text="backend.managed ? '编辑存储后端' : '该后端由外部配置管理'">
          <UButton
            icon="i-tabler-pencil"
            color="neutral"
            variant="ghost"
            square
            size="sm"
            :disabled="!backend.managed"
            :aria-label="`编辑存储后端：${backend.name}`"
            @click="emit('edit', backend)"
          />
        </UTooltip>
      </div>
    </div>
  </section>
</template>
