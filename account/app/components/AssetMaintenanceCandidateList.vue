<script setup lang="ts">
import { ManageEmpty } from '@platform/manage/components'
import type { AssetItem } from '~/types/asset-admin'

const props = withDefaults(defineProps<{
  items?: AssetItem[]
  candidates?: number
  limit: number
  kind: 'prune' | 'migration' | 'rebuild'
  emptyIcon: string
  emptyText: string
}>(), {
  items: () => [],
  candidates: 0
})

function formatBytes(value: number) {
  if (!value) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function briefDate(value?: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}

function itemMeta(item: AssetItem) {
  if (props.kind === 'prune') {
    return `${item.spaceKey || 'default'} / ${item.siteKey} / ${item.profileKey} · ${formatBytes(item.size)} · ${briefDate(item.createdAt)}`
  }
  if (props.kind === 'migration') return `${item.storageBackend} · ${item.mime} · ${formatBytes(item.size)}`
  return `${item.mime} · ${formatBytes(item.size)} · ${briefDate(item.createdAt)}`
}
</script>

<template>
  <div class="overflow-hidden rounded-lg border border-default bg-default">
    <div class="border-b border-default px-3 py-2 text-sm text-muted">
      候选 {{ candidates }} 个，本次最多处理 {{ limit }} 个
    </div>
    <ManageEmpty v-if="!items.length" :icon="emptyIcon" :text="emptyText" />
    <div v-else class="max-h-72 divide-y divide-default overflow-y-auto overflow-x-hidden">
      <div v-for="asset in items" :key="asset.id" class="flex min-w-0 items-center gap-3 px-3 py-2.5">
        <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
          <UIcon :name="kind === 'rebuild' ? 'i-tabler-photo' : kind === 'migration' ? 'i-tabler-file-database' : 'i-tabler-file'" class="size-4" />
        </span>
        <div class="min-w-0 flex-1">
          <div class="line-clamp-1 text-sm font-medium text-highlighted">{{ asset.filename || asset.id }}</div>
          <div class="line-clamp-1 text-xs text-muted">{{ itemMeta(asset) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
