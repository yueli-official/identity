<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import { ManageCollectionToolbar, ManageEmpty, ManageRowShell, ManageSortDirectionButton, ManageViewToggle } from '@platform/manage/components'
import type { AssetItem, AssetMaintenanceTask, AssetSite } from '~/types/asset-admin'

type SelectOption = { label: string, value: string }

const {
  assets,
  total,
  filterCount,
  spaceOptions,
  siteOptions,
  profileOptions,
  visibilityOptions,
  mimeOptions,
  sortOptions,
  selectedIds,
  pageSelected,
  pageIndeterminate,
  pageSizeItems,
  totalPages,
  sites,
  queueing = false,
  queueError = '',
  taskId = '',
  task,
  controllingTaskId = '',
  actionsFor
} = defineProps<{
  assets: AssetItem[]
  total: number
  filterCount: number
  spaceOptions: SelectOption[]
  siteOptions: SelectOption[]
  profileOptions: SelectOption[]
  visibilityOptions: SelectOption[]
  mimeOptions: SelectOption[]
  sortOptions: SelectOption[]
  selectedIds: string[]
  pageSelected: boolean
  pageIndeterminate: boolean
  pageSizeItems: Array<{ label: string, value: number }>
  totalPages: number
  sites: AssetSite[]
  queueing?: boolean
  queueError?: string
  taskId?: string
  task?: AssetMaintenanceTask
  controllingTaskId?: string
  actionsFor: (asset: AssetItem) => DropdownMenuItem[][]
}>()

const emit = defineEmits<{
  toggleOne: [id: string]
  togglePage: [value: boolean]
  queueSelected: []
  clearSelection: []
  taskAction: [action: 'pause' | 'resume' | 'cancel']
  openMaintenance: []
  dismissTask: []
}>()

const search = defineModel<string>('search', { required: true })
const spaceKey = defineModel<string>('spaceKey', { required: true })
const siteKey = defineModel<string>('siteKey', { required: true })
const profileKey = defineModel<string>('profileKey', { required: true })
const visibility = defineModel<string>('visibility', { required: true })
const mime = defineModel<string>('mime', { required: true })
const sort = defineModel<string>('sort', { required: true })
const direction = defineModel<'asc' | 'desc'>('direction', { required: true })
const view = defineModel<'grid' | 'list'>('view', { required: true })
const page = defineModel<number>('page', { required: true })
const pageSize = defineModel<number>('pageSize', { required: true })

const selected = computed(() => new Set(selectedIds))

function isSelected(id: string) {
  return selected.value.has(id)
}

function siteName(key: string) {
  return sites.find(site => site.siteKey === key)?.name || key
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
</script>

<template>
  <section class="space-y-4" aria-label="中央素材库">
    <ManageCollectionToolbar v-model:search="search" search-placeholder="搜索文件名、标题、替代文本或 ID…" :filter-count="filterCount">
      <template #filters>
        <USelectMenu v-model="spaceKey" :items="spaceOptions" value-key="value" :search-input="{ placeholder: '搜索资源空间…' }" />
        <USelectMenu v-model="siteKey" :items="siteOptions" value-key="value" :search-input="{ placeholder: '搜索站点…' }" />
        <USelectMenu v-model="profileKey" :items="profileOptions" value-key="value" :search-input="{ placeholder: '搜索 Profile…' }" />
        <USelect v-model="visibility" :items="visibilityOptions" value-key="value" />
        <USelect v-model="mime" :items="mimeOptions" value-key="value" />
        <USelect v-model="sort" :items="sortOptions" value-key="value" icon="i-tabler-arrows-sort" />
        <ManageSortDirectionButton v-model="direction" />
      </template>
      <template #actions>
        <ManageViewToggle v-model="view" :items="[
          { key: 'grid', label: '网格', icon: 'i-tabler-layout-grid' },
          { key: 'list', label: '列表', icon: 'i-tabler-list' }
        ]" />
      </template>
    </ManageCollectionToolbar>

    <ManageEmpty v-if="!assets.length" icon="i-tabler-photo-off" text="没有匹配的资源" />

    <div v-else-if="view === 'grid'" class="grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(min(14rem,100%),1fr))]">
      <article
        v-for="asset in assets"
        :key="asset.id"
        class="group relative overflow-hidden rounded-xl border border-default bg-default transition hover:-translate-y-0.5 hover:shadow-sm"
        :class="isSelected(asset.id) ? 'ring-2 ring-primary' : ''"
      >
        <div class="relative aspect-[4/3] overflow-hidden border-b border-default bg-elevated">
          <img v-if="asset.cdnUrl && asset.mime.startsWith('image/')" :src="asset.cdnUrl" :alt="asset.filename" loading="lazy" class="size-full object-cover transition duration-300 group-hover:scale-[1.03]">
          <div v-else class="grid size-full place-items-center">
            <UIcon name="i-tabler-file" class="size-9 text-muted" />
          </div>
          <UCheckbox
            class="absolute left-2 top-2 rounded-md bg-default/90 p-1 backdrop-blur"
            :model-value="isSelected(asset.id)"
            :aria-label="`选择素材：${asset.filename || asset.id}`"
            @update:model-value="emit('toggleOne', asset.id)"
          />
          <UDropdownMenu :items="actionsFor(asset)">
            <UButton icon="i-tabler-dots-vertical" color="neutral" variant="solid" square size="xs" class="absolute right-2 top-2" :aria-label="`素材操作：${asset.filename || asset.id}`" />
          </UDropdownMenu>
        </div>
        <div class="min-w-0 p-3">
          <h2 class="truncate text-sm font-semibold text-highlighted">{{ asset.filename || asset.id }}</h2>
          <p class="mt-1 truncate text-xs text-muted">{{ siteName(asset.siteKey) }} · {{ asset.profileKey || 'default' }}</p>
          <div class="mt-3 flex items-end justify-between gap-3 border-t border-default pt-2.5 text-xs text-muted">
            <div class="min-w-0">
              <p class="truncate">{{ formatBytes(asset.size) }}<span v-if="asset.width && asset.height"> · {{ asset.width }}×{{ asset.height }}</span></p>
              <p class="mt-0.5 truncate">{{ asset.storageBackend || 'local' }}</p>
            </div>
            <span v-if="asset.refCount" class="shrink-0 text-warning">{{ asset.refCount }} 引用</span>
          </div>
        </div>
      </article>
    </div>

    <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
      <ManageRowShell
        v-for="asset in assets"
        :key="asset.id"
        :selected="isSelected(asset.id)"
        :selection-label="`选择素材：${asset.filename || asset.id}`"
        @select="emit('toggleOne', asset.id)"
      >
        <template #media>
          <div class="grid size-14 shrink-0 place-items-center overflow-hidden rounded-lg bg-elevated">
            <img v-if="asset.cdnUrl && asset.mime.startsWith('image/')" :src="asset.cdnUrl" :alt="asset.filename" loading="lazy" class="size-full object-cover">
            <UIcon v-else name="i-tabler-file" class="size-5 text-muted" />
          </div>
        </template>
        <div class="min-w-0">
          <p class="truncate text-sm font-semibold text-highlighted">{{ asset.filename || asset.id }}</p>
          <p class="mt-0.5 truncate text-xs text-muted">{{ asset.mime }} · {{ formatBytes(asset.size) }}<span v-if="asset.width && asset.height"> · {{ asset.width }}×{{ asset.height }}</span></p>
          <p class="mt-1 truncate text-xs text-dimmed">空间 {{ asset.spaceKey || 'default' }} · {{ siteName(asset.siteKey) }} / {{ asset.profileKey }}</p>
        </div>
        <template #meta>
          <div class="min-w-0 text-xs md:w-44 md:text-right">
            <p class="truncate text-default">{{ asset.storageBackend || 'local' }}</p>
            <p class="mt-0.5 truncate text-muted">{{ asset.visibility }} · {{ asset.deliveryPolicy || 'public' }}</p>
            <p v-if="asset.refCount" class="mt-1 text-warning">{{ asset.refCount }} 个引用</p>
          </div>
        </template>
        <template #actions>
          <UDropdownMenu :items="actionsFor(asset)">
            <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="sm" :aria-label="`素材操作：${asset.filename || asset.id}`" />
          </UDropdownMenu>
        </template>
      </ManageRowShell>
    </div>

    <AssetMaintenanceDock
      v-if="total > 0 || assets.length"
      v-model:page="page"
      v-model:page-size="pageSize"
      :total-pages="totalPages"
      :page-size-items="pageSizeItems"
      :total="total"
      :selected-count="selectedIds.length"
      :page-selected="pageSelected"
      :page-indeterminate="pageIndeterminate"
      :queueing="queueing"
      :queue-error="queueError"
      :task-id="taskId"
      :task="task"
      :controlling-task-id="controllingTaskId"
      @toggle-page="emit('togglePage', $event)"
      @queue-selected="emit('queueSelected')"
      @clear-selection="emit('clearSelection')"
      @task-action="emit('taskAction', $event)"
      @open-maintenance="emit('openMaintenance')"
      @dismiss-task="emit('dismissTask')"
    />
  </section>
</template>
