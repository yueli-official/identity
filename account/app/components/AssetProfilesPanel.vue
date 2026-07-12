<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import { ManageEmpty } from '@platform/manage/components'
import type { AssetProfile, AssetSite, AssetVariant } from '~/types/asset-admin'

const { profiles, variants, sites, profileActions, variantActions } = defineProps<{
  profiles: AssetProfile[]
  variants: AssetVariant[]
  sites: AssetSite[]
  profileActions: (profile: AssetProfile) => DropdownMenuItem[][]
  variantActions: (variant: AssetVariant) => DropdownMenuItem[][]
}>()

const emit = defineEmits<{ addVariant: [profile: AssetProfile] }>()

function variantsFor(profile: AssetProfile) {
  return variants.filter(variant => variant.siteKey === profile.siteKey && variant.profileKey === profile.profileKey)
}

function siteName(key: string) {
  return sites.find(site => site.siteKey === key)?.name || key
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function storageText(profile: AssetProfile) {
  return profile.storageBackend || '继承站点默认'
}

function accessText(profile: AssetProfile) {
  return profile.defaultVisibility === 'private' ? '私有 · 签名链接' : '公开 · 公开直链'
}
</script>

<template>
  <section class="space-y-4" aria-labelledby="asset-profiles-heading">
    <div>
      <h2 id="asset-profiles-heading" class="text-sm font-semibold text-highlighted">Profile 与派生规格</h2>
      <p class="mt-1 text-xs text-muted">按站点管理上传用途、文件限制、访问级别和命名派生规格。</p>
    </div>
    <ManageEmpty v-if="!profiles.length" icon="i-tabler-folder-off" text="还没有 Profile" />
    <div v-else class="grid gap-4 lg:grid-cols-2">
      <article v-for="profile in profiles" :key="`${profile.siteKey}:${profile.profileKey}`" class="rounded-lg border border-default bg-default">
        <div class="flex items-start justify-between gap-3 border-b border-default p-4">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <UIcon name="i-tabler-folder-cog" class="size-4 text-primary" />
              <h3 class="truncate text-sm font-semibold text-highlighted">{{ profile.profileKey }}</h3>
            </div>
            <p class="mt-1 truncate text-xs text-muted">{{ siteName(profile.siteKey) }} · {{ profile.purpose || '未填写用途' }}</p>
          </div>
          <div class="flex items-center gap-1">
            <UBadge :label="`${profile.assetCount} 素材`" color="neutral" variant="soft" />
            <UDropdownMenu :items="profileActions(profile)">
              <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="sm" :aria-label="`Profile 操作：${profile.profileKey}`" />
            </UDropdownMenu>
          </div>
        </div>
        <div class="space-y-3 p-4 text-sm">
          <dl class="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
            <div><dt class="inline text-muted">类型 </dt><dd class="inline text-default">{{ profile.allowedExt }}</dd></div>
            <div><dt class="inline text-muted">上限 </dt><dd class="inline text-default">{{ formatBytes(profile.maxSizeBytes) }}</dd></div>
            <div><dt class="inline text-muted">后端 </dt><dd class="inline text-default">{{ storageText(profile) }}</dd></div>
            <div><dt class="inline text-muted">访问级别 </dt><dd class="inline text-default">{{ accessText(profile) }}</dd></div>
          </dl>
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-medium text-muted">Variant · {{ profile.variantCount }}</span>
            <UButton icon="i-tabler-plus" label="添加规格" color="neutral" variant="soft" size="sm" @click="emit('addVariant', profile)" />
          </div>
          <div v-if="!variantsFor(profile).length" class="rounded-md border border-dashed border-default px-3 py-2 text-xs text-muted">还没有派生规格。</div>
          <div v-else class="space-y-1.5">
            <div v-for="variant in variantsFor(profile)" :key="variant.id" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md bg-elevated/40 px-3 py-2 text-xs">
              <div class="min-w-0">
                <span class="font-medium text-default">{{ variant.variantKey }}</span>
                <span class="ml-2 text-muted">{{ variant.width }}×{{ variant.height }} · {{ variant.mode }} · v{{ variant.version }}</span>
              </div>
              <UDropdownMenu :items="variantActions(variant)">
                <UButton icon="i-tabler-dots" color="neutral" variant="ghost" square size="sm" :aria-label="`Variant 操作：${variant.variantKey}`" />
              </UDropdownMenu>
            </div>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
