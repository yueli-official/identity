<script setup lang="ts">
import { ManageEmpty } from '@platform/manage/components'
import type { AssetSite } from '~/types/asset-admin'

defineProps<{ sites: AssetSite[] }>()
const emit = defineEmits<{ edit: [site: AssetSite] }>()
</script>

<template>
  <section class="space-y-4" aria-labelledby="asset-sites-heading">
    <div>
      <h2 id="asset-sites-heading" class="text-sm font-semibold text-highlighted">站点</h2>
      <p class="mt-1 text-xs text-muted">每个站点拥有自己的默认存储后端和 Profile/Variant 计数。</p>
    </div>
    <ManageEmpty v-if="!sites.length" icon="i-tabler-world-off" text="还没有站点配置" />
    <div v-else class="grid gap-3 lg:grid-cols-2">
      <article v-for="site in sites" :key="site.siteKey" class="rounded-lg border border-default bg-default p-4">
        <div class="flex items-start justify-between gap-3">
          <div class="flex min-w-0 items-center gap-2">
            <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
              <UIcon name="i-tabler-world" class="size-4" />
            </span>
            <div class="min-w-0">
              <h3 class="truncate text-sm font-semibold text-highlighted">{{ site.name }}</h3>
              <p class="truncate font-mono text-xs text-muted">{{ site.siteKey }}</p>
            </div>
          </div>
          <div class="flex items-center gap-1">
            <UBadge :label="site.enabled ? '启用' : '停用'" :color="site.enabled ? 'success' : 'neutral'" variant="soft" />
            <UTooltip text="编辑站点">
              <UButton icon="i-tabler-pencil" color="neutral" variant="ghost" square size="sm" :aria-label="`编辑站点：${site.name}`" @click="emit('edit', site)" />
            </UTooltip>
          </div>
        </div>
        <div class="mt-4 grid grid-cols-3 gap-2 text-xs">
          <div v-for="metric in [{ label: '素材', value: site.assetCount }, { label: 'Profile', value: site.profileCount }, { label: 'Variant', value: site.variantCount }]" :key="metric.label" class="rounded-md bg-elevated/40 px-3 py-2">
            <div class="text-muted">{{ metric.label }}</div>
            <div class="mt-0.5 font-semibold text-highlighted tabular-nums">{{ metric.value }}</div>
          </div>
        </div>
        <div class="mt-3 truncate text-xs text-muted">默认后端 <span class="font-mono text-default">{{ site.defaultStorageBackend }}</span></div>
      </article>
    </div>
  </section>
</template>
