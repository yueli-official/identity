<script setup lang="ts">
import { ManageUserMenu } from "~/components/manage";
import type {
  AdminNavigationItem,
  AdminSearchGroup,
  AdminShellMessages,
} from "@yueli/ui/admin";

const route = useRoute();
const { me, logout, isAdmin } = useSession();

const messages: AdminShellMessages = {
  skipToContent: "跳到主要内容",
  search: "搜索账户中心",
  searchPlaceholder: "搜索账户设置",
  currentLocation: "当前位置",
};

const navigation = computed<readonly AdminNavigationItem[]>(() => [
  { label: "个人资料", icon: "i-tabler-user-edit", to: "/", active: route.path === "/" },
  { label: "账户安全", icon: "i-tabler-shield-lock", to: "/security", active: route.path === "/security" },
  { label: "登录方式", icon: "i-tabler-key", to: "/connections", active: route.path === "/connections" },
  { label: "登录会话", icon: "i-tabler-devices", to: "/sessions", active: route.path === "/sessions" },
  { label: "开发者令牌", icon: "i-tabler-api", to: "/developer-tokens", active: route.path === "/developer-tokens" },
]);

const currentLabel = computed(
  () => navigation.value.find((item) => item.active)?.label || "个人资料",
);

const searchGroups = computed<readonly AdminSearchGroup[]>(() => [
  {
    id: "account-center-pages",
    label: "账户设置",
    items: navigation.value.map((item, index) => ({
      id: `account-center-page-${index}`,
      label: item.label,
      icon: item.icon,
      to: item.to,
    })),
  },
]);

async function handleLogout() {
  await logout();
  await navigateTo("/login");
}
</script>

<template>
  <!--
    THESIS: 账户设置是一组可定位的任务，不是一张无限向下延伸的资料表。
    OWN-WORLD: 沿用 Account 青绿色信任信号、语义表面、共享管理壳与克制边界。
    STORY: 用户先定位资料、安全、登录方式或会话，再只处理当前主题。
    FIRST VIEWPORT: 固定分区导航、当前位置和单一页标题共同建立方向，内容区只出现一个任务组。
    FORM: 共享后台壳中的个人设置工作区；稳定路由代替单页锚点和平铺卡片。
    FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
  -->
  <YAdminConsoleLayout
    :navigation="navigation"
    :search-groups="searchGroups"
    :messages="messages"
    storage-key="account-center"
    main-id="account-main"
    brand-label="账户中心"
    brand-icon="i-tabler-shield-check"
    brand-to="/"
    context-label="账户中心"
    :current-label="currentLabel"
    back-to-top-label="返回顶部"
    class="account-center-shell"
    data-account-center-shell
  >
    <template #account="{ collapsed }">
      <ManageUserMenu
        v-if="me"
        :name="me.displayName"
        :email="me.email"
        :avatar-url="me.avatarUrl || ''"
        :home-to="isAdmin ? '/admin/platform' : ''"
        home-label="管理控制台"
        show-appearance
        :trigger-mode="collapsed ? 'collapsed' : 'sidebar'"
        :logout="handleLogout"
      />
    </template>
    <slot />
  </YAdminConsoleLayout>

  <AdminStepUpModal />
</template>
