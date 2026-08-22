<script setup lang="ts">
import { ManageUserMenu } from "~/components/manage";
import type {
  AdminNavigationItem,
  AdminSearchGroup,
  AdminShellMessages,
} from "@yueli/ui/admin";

const route = useRoute();
const { me, logout } = useSession();
const sidebarOpen = ref(false);

const messages: AdminShellMessages = {
  skipToContent: "跳到主要内容",
  search: "搜索管理控制台",
  searchPlaceholder: "搜索页面与常用操作",
};

function closeSidebar() {
  sidebarOpen.value = false;
}

function active(path: string, exact = false) {
  return exact ? route.path === path : route.path.startsWith(path);
}

const navigation = computed<readonly AdminNavigationItem[]>(() => [
  {
    label: "平台能力",
    icon: "i-tabler-activity-heartbeat",
    to: "/admin/platform",
    active: active("/admin/platform"),
    onSelect: closeSidebar,
  },
  {
    label: "用户与权限",
    icon: "i-tabler-users",
    to: "/admin/users",
    active: active("/admin/users"),
    onSelect: closeSidebar,
  },
  {
    label: "资源运营",
    icon: "i-tabler-photo-cog",
    to: "/admin/assets",
    active: active("/admin/assets"),
    onSelect: closeSidebar,
  },
]);

const secondaryNavigation = computed<readonly AdminNavigationItem[]>(() => [
  {
    label: "账户中心",
    icon: "i-tabler-user-circle",
    to: "/",
    active: active("/", true),
    onSelect: closeSidebar,
  },
]);

const searchGroups = computed<readonly AdminSearchGroup[]>(() => [
  {
    id: "account-admin-pages",
    label: "管理页面",
    items: navigation.value.map((item, index) => ({
      id: `account-admin-page-${index}`,
      label: item.label,
      icon: item.icon,
      to: item.to,
    })),
  },
  {
    id: "account-admin-actions",
    label: "常用操作",
    items: [
      {
        id: "account-admin-user-search",
        label: "查找用户",
        icon: "i-tabler-user-search",
        to: "/admin/users",
      },
      {
        id: "account-admin-asset-security",
        label: "处理资源安全队列",
        icon: "i-tabler-shield-search",
        to: "/admin/assets?section=security",
      },
    ],
  },
]);

async function handleLogout() {
  await logout();
  await navigateTo("/login");
}
</script>

<template>
  <YAdminShell
      v-model:open="sidebarOpen"
      :navigation="navigation"
      :secondary-navigation="secondaryNavigation"
      :search-groups="searchGroups"
      :messages="messages"
      storage-key="account-admin"
      main-id="admin-main"
      :default-size="16"
      :min-size="14"
      :max-size="20"
    >
      <template #brand="{ collapsed }">
        <NuxtLink
          to="/admin/platform"
          :aria-label="collapsed ? '管理控制台' : undefined"
          :class="[
            'flex min-h-11 items-center gap-2 rounded-md px-1.5 text-highlighted transition hover:bg-elevated',
            collapsed ? 'justify-center' : 'w-full',
          ]"
          @click="closeSidebar"
        >
          <span
            class="grid size-7 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"
          >
            <UIcon name="i-tabler-shield-cog" class="size-4" />
          </span>
          <span
            v-if="!collapsed"
            class="min-w-0 truncate text-sm font-semibold"
          >
            管理控制台
          </span>
        </NuxtLink>
      </template>

      <template #sidebar-footer="{ collapsed }">
        <ManageUserMenu
          v-if="me"
          :name="me.displayName"
          :email="me.email"
          :avatar-url="me.avatarUrl || ''"
          home-to="/"
          home-label="账户中心"
          show-appearance
          :trigger-mode="collapsed ? 'collapsed' : 'sidebar'"
          :logout="handleLogout"
        />
      </template>

      <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
        <UDashboardNavbar
          title="管理控制台"
          class="shrink-0 lg:hidden"
        />

        <main
          id="admin-main"
          tabindex="-1"
          class="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 outline-none sm:p-6"
        >
          <slot />
        </main>

        <YBackToTop
          target-id="admin-main"
          scroll-container-id="admin-main"
          avoid-selector="[data-manage-dock], [data-back-to-top-avoid]"
          label="返回顶部"
        />
      </div>
  </YAdminShell>

  <AdminStepUpModal />
</template>
