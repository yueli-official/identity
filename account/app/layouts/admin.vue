<script setup lang="ts">
import { ManageUserMenu } from "~/components/manage";
import type {
  AdminNavigationItem,
  AdminSearchGroup,
  AdminShellMessages,
} from "@yueli/ui/admin";

const route = useRoute();
const { me, logout } = useSession();

const messages: AdminShellMessages = {
  skipToContent: "跳到主要内容",
  search: "搜索管理控制台",
  searchPlaceholder: "搜索页面与常用操作",
  currentLocation: "当前位置",
};

function active(path: string, exact = false) {
  return exact ? route.path === path : route.path.startsWith(path);
}

const navigation = computed<readonly AdminNavigationItem[]>(() => [
  {
    label: "平台能力",
    icon: "i-tabler-activity-heartbeat",
    to: "/admin/platform",
    active: active("/admin/platform"),
  },
  {
    label: "用户与权限",
    icon: "i-tabler-users",
    to: "/admin/users",
    active: active("/admin/users"),
  },
  {
    label: "资源运营",
    icon: "i-tabler-photo-cog",
    to: "/admin/assets",
    active: active("/admin/assets"),
  },
  {
    label: "登录配置",
    icon: "i-tabler-login-2",
    to: "/admin/login-providers",
    active: active("/admin/login-providers"),
  },
]);

const currentLabel = computed(() => {
  if (route.path === "/admin/users") return "用户管理";
  if (route.path === "/admin/assets") return "资源管理";
  if (route.path === "/admin/platform") return "平台状态";
  if (route.path === "/admin/login-providers") return "登录配置";
  const service = route.path.match(/^\/admin\/platform\/([^/]+)\/?$/u)?.[1];
  if (service) {
    return {
      identity: "Identity",
      asset: "Asset",
      commerce: "Commerce",
      notification: "Notification",
    }[service] || "平台状态";
  }
  return "管理控制台";
});

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
  <YAdminConsoleLayout
    :navigation="navigation"
    :search-groups="searchGroups"
    :messages="messages"
    storage-key="account-admin"
    main-id="admin-main"
    brand-label="管理控制台"
    brand-icon="i-tabler-shield-cog"
    brand-to="/admin/platform"
    context-label="管理控制台"
    :current-label="currentLabel"
    back-to-top-label="返回顶部"
    class="account-admin-shell"
    data-account-admin-shell
  >
    <template #account="{ collapsed }">
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
    <slot />
  </YAdminConsoleLayout>

  <AdminStepUpModal />
</template>
