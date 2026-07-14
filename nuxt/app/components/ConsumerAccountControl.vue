<script setup lang="ts">
import { computed } from "vue";
import { PlatformUserMenu } from "@platform/ui/components";
import type { PlatformUserMenuAction } from "@platform/ui/components";

const props = withDefaults(
  defineProps<{
    contextActions?: readonly PlatformUserMenuAction[];
    manageTo?: string;
    manageLabel?: string;
    homeTo?: string;
    homeLabel?: string;
    loginLabel?: string;
  }>(),
  {
    contextActions: () => [],
    manageTo: "",
    manageLabel: "控制台",
    homeTo: "",
    homeLabel: "返回主站",
    loginLabel: "登录",
  },
);

const { user, loggedIn, isAdmin, login, logout } = useAuth();
const accountUrl = computed(
  () => useRuntimeConfig().public.accountUrl || "http://localhost:3000",
);

const resolvedContextActions = computed<PlatformUserMenuAction[]>(() => [
  ...props.contextActions,
  ...(props.manageTo && isAdmin.value
    ? [
        {
          label: props.manageLabel,
          icon: "i-tabler-layout-dashboard",
          to: props.manageTo,
        },
      ]
    : []),
]);

const utilityActions = computed<PlatformUserMenuAction[]>(() => [
  ...(props.homeTo
    ? [
        {
          label: props.homeLabel,
          icon: "i-tabler-arrow-back-up",
          to: props.homeTo,
        },
      ]
    : []),
  {
    label: "用户设置",
    icon: "i-tabler-user-cog",
    onSelect: async () => {
      await navigateTo(accountUrl.value, { external: true });
    },
  },
]);

async function handleLogin(): Promise<void> {
  await login();
}
</script>

<template>
  <PlatformUserMenu
    v-if="loggedIn"
    :name="user?.name"
    :email="user?.email"
    :avatar-url="user?.avatar"
    :context-actions="resolvedContextActions"
    :utility-actions
    :logout
  />
  <UButton
    v-else
    color="neutral"
    variant="ghost"
    icon="i-tabler-login-2"
    :label="loginLabel"
    @click="handleLogin"
  />
</template>
