<script setup lang="ts">
const route = useRoute()
const navItems = [
  { label: '平台', to: '/admin/platform', icon: 'i-tabler-activity-heartbeat' },
  { label: '用户', to: '/admin/users', icon: 'i-tabler-users' },
  { label: '资源', to: '/admin/assets', icon: 'i-tabler-photo-cog' },
]

function isActive(to: string) {
  return route.path === to || route.path.startsWith(`${to}/`)
}
</script>

<template>
  <div class="flex h-full flex-col bg-elevated/30">
    <NuxtLink to="/admin/platform" class="font-display flex h-16 items-center gap-2 border-b border-default px-5 font-semibold text-highlighted">
      <span class="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary">
        <UIcon name="i-tabler-shield-cog" class="size-5" />
      </span>
      管理控制台
    </NuxtLink>
    <nav aria-label="管理控制台" class="flex-1 space-y-1 p-3">
      <NuxtLink
        v-for="item in navItems"
        :key="item.to"
        :to="item.to"
        class="flex min-h-11 items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition"
        :class="isActive(item.to) ? 'bg-primary/10 text-primary' : 'text-muted hover:bg-elevated hover:text-default'"
      >
        <UIcon :name="item.icon" class="size-5 shrink-0" />
        <span class="truncate">{{ item.label }}</span>
      </NuxtLink>
    </nav>
  </div>
</template>
