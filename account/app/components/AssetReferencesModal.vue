<script setup lang="ts">
import { ManageEmpty, SkeletonList } from '@platform/manage/components'
import type { AssetReference } from '~/types/asset-admin'

withDefaults(defineProps<{
  assetId?: string
  references?: AssetReference[]
  loading?: boolean
}>(), {
  assetId: '',
  references: () => [],
  loading: false
})

const open = defineModel<boolean>('open', { required: true })

function openExternal(url: string) {
  if (url) window.open(url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <UModal v-model:open="open" title="素材引用" description="查看阻止素材删除的业务引用。">
    <template #body>
      <div class="space-y-3">
        <p class="line-clamp-1 font-mono text-xs text-dimmed">{{ assetId }}</p>
        <SkeletonList v-if="loading" :rows="3" />
        <ManageEmpty v-else-if="!references.length" icon="i-tabler-link-off" text="还没有引用记录" />
        <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
          <div v-for="reference in references" :key="reference.id" class="flex min-w-0 items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
            <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
              <UIcon name="i-tabler-link" class="size-4" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="line-clamp-1 text-sm font-medium text-highlighted">{{ reference.refLabel || reference.refId }}</div>
              <div class="line-clamp-1 text-xs text-muted">{{ reference.siteKey }} · {{ reference.refType }} · {{ reference.refId }}</div>
            </div>
            <UButton
              v-if="reference.refUrl"
              icon="i-tabler-external-link"
              color="neutral"
              variant="ghost"
              square
              size="xs"
              :aria-label="`打开 ${reference.refLabel || reference.refId}`"
              @click="openExternal(reference.refUrl)"
            />
          </div>
        </div>
      </div>
    </template>
  </UModal>
</template>
