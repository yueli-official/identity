<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

const { me, logout, isAdmin } = useSession()

const initial = computed(() =>
  (me.value?.displayName || me.value?.email || '?').charAt(0).toUpperCase()
)

const userItems = computed<DropdownMenuItem[][]>(() => [
  [{ label: me.value?.displayName || me.value?.email || '', type: 'label' }],
  [
    { label: '账户中心', icon: 'i-tabler-user-circle', to: '/' },
    // 仅站群超级管理员可见:进入用户管理控制台。
    ...(isAdmin.value ? [{ label: '管理控制台', icon: 'i-tabler-shield-cog', to: '/admin/users' }] : []),
    {
      label: '退出登录',
      icon: 'i-tabler-logout',
      onSelect: async () => {
        await logout()
        await navigateTo('/login')
      }
    }
  ]
])
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-default text-default">
    <header class="sticky top-0 z-20 border-b border-default bg-default/75 backdrop-blur">
      <div class="mx-auto flex h-16 w-full max-w-3xl items-center justify-between gap-4 px-4">
        <NuxtLink
          to="/"
          class="font-display flex items-center gap-2 text-base font-semibold text-highlighted"
        >
          <span class="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary">
            <UIcon name="i-tabler-shield-check" class="size-5" />
          </span>
          账户中心
        </NuxtLink>

        <div class="flex items-center gap-1.5">
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

    <main class="mx-auto w-full max-w-3xl flex-1 px-4 py-8 sm:py-10">
      <slot />
    </main>
  </div>
</template>
