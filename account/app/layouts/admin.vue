<script setup lang="ts">
import { ManageShell, ManageUserMenu } from '@platform/manage/components'

const route = useRoute()
const { me, logout } = useSession()

const contextLabel = computed(() => {
  if (route.path === '/admin/platform') return '平台状态'
  if (route.path.startsWith('/admin/platform/')) return '服务详情'
  if (route.path === '/admin/users') return '用户'
  if (route.path === '/admin/assets') return '资源'
  return '管理控制台'
})
const handleLogout = async () => {
  await logout()
  await navigateTo('/login')
}
</script>

<template>
  <ManageShell
    site-name="管理控制台"
    :context-label="contextLabel"
    home-to="/admin/platform"
    storage-key="account-admin"
    :show-back-to-top="route.path.startsWith('/admin/platform')"
  >
    <template #sidebar><AdminSidebar /></template>
    <template #user>
      <ManageUserMenu
        v-if="me"
        :name="me.displayName"
        :email="me.email"
        :avatar-url="me.avatarUrl || ''"
        home-to="/"
        home-label="账户中心"
        settings-to="/#profile-settings"
        :settings-external="false"
        :logout="handleLogout"
      />
    </template>
    <slot />
    <AdminStepUpModal />
  </ManageShell>
</template>
