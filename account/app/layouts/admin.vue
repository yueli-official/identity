<script setup lang="ts">
// Admin console shell: a wider canvas than the personal account center (lists +
// tables need room) with a sticky top bar (brand + back-to-account + user).
// Single section for now (用户管理); add nav items here if more land.
const { me, logout } = useSession()
const initial = computed(() =>
  (me.value?.displayName || me.value?.email || '?').charAt(0).toUpperCase()
)
const userItems = computed(() => [
  [{ label: me.value?.displayName || me.value?.email || '', type: 'label' as const }],
  [
    { label: '账户中心', icon: 'i-tabler-user-circle', to: '/' },
    {
      label: '退出登录',
      icon: 'i-tabler-logout',
      onSelect: async () => { await logout(); await navigateTo('/login') }
    }
  ]
])
const navItems = [
  { label: '用户', to: '/admin/users', icon: 'i-tabler-users' },
  { label: '资源', to: '/admin/assets', icon: 'i-tabler-photo-cog' }
]
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-default text-default">
    <header class="sticky top-0 z-20 border-b border-default bg-default/75 backdrop-blur">
      <div class="mx-auto flex h-16 w-full max-w-6xl items-center justify-between gap-4 px-4 lg:px-6">
        <div class="flex items-center gap-2">
          <NuxtLink
            to="/admin/users"
            class="font-display flex items-center gap-2 text-base font-semibold text-highlighted"
          >
            <span class="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary">
              <UIcon name="i-tabler-shield-cog" class="size-5" />
            </span>
            管理控制台
          </NuxtLink>
          <nav class="ml-4 hidden items-center gap-1 md:flex">
            <UButton
              v-for="item in navItems"
              :key="item.to"
              :to="item.to"
              :icon="item.icon"
              :label="item.label"
              color="neutral"
              variant="ghost"
              size="sm"
            />
          </nav>
        </div>

        <div class="flex items-center gap-1.5">
          <UButton to="/" icon="i-tabler-arrow-back-up" label="账户中心" color="neutral" variant="ghost" size="sm" />
          <UColorModeButton />
          <UDropdownMenu v-if="me" :items="userItems" :ui="{ content: 'w-52' }">
            <UButton variant="ghost" color="neutral" class="gap-2 px-1.5">
              <UAvatar :text="initial" :src="me.avatarUrl || undefined" size="xs" />
              <span class="hidden max-w-32 truncate text-sm sm:block">{{ me.displayName || me.email }}</span>
            </UButton>
          </UDropdownMenu>
        </div>
      </div>
    </header>

    <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8 lg:px-6 lg:py-10">
      <slot />
    </main>
  </div>
</template>
