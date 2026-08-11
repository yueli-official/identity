<script setup lang="ts">
import { computed } from "vue";
import {
  AccountMenu,
  type AccountMenuAction,
  type AccountMenuTriggerMode,
} from "@yueli/ui/account-menu/pattern";
import { identityAccountMenuMessages } from "../utils/account-menu";

const props = withDefaults(
  defineProps<{
    contextActions?: readonly AccountMenuAction[];
    manageTo?: string;
    manageLabel?: string;
    homeTo?: string;
    homeLabel?: string;
    loginLabel?: string;
    triggerMode?: AccountMenuTriggerMode;
  }>(),
  {
    contextActions: () => [],
    manageTo: "",
    manageLabel: "控制台",
    homeTo: "",
    homeLabel: "返回主站",
    loginLabel: "登录",
    triggerMode: "inline",
  },
);

const { user, loggedIn, isAdmin, login, logout } = useAuth();
const accountUrl = computed(
  () => useRuntimeConfig().public.accountUrl || "http://localhost:3000",
);

const resolvedContextActions = computed<AccountMenuAction[]>(() => [
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

const utilityActions = computed<AccountMenuAction[]>(() => [
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
  <AccountMenu
    v-if="loggedIn"
    :name="user?.name"
    :email="user?.email"
    :avatar-url="user?.avatar"
    :context-actions="resolvedContextActions"
    :utility-actions
    :logout
    :messages="identityAccountMenuMessages"
    :trigger-mode="triggerMode"
  />
  <UButton
    v-else
    color="neutral"
    variant="ghost"
    icon="i-tabler-login-2"
    :label="triggerMode === 'collapsed' ? undefined : loginLabel"
    :square="triggerMode === 'collapsed'"
    :aria-label="triggerMode === 'collapsed' ? loginLabel : undefined"
    @click="handleLogin"
  />
</template>
