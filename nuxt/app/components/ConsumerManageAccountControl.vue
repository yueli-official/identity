<script setup lang="ts">
import { computed } from "vue";
import {
  AccountMenu,
  type AccountMenuAction,
  type AccountMenuAppearance,
  type AccountMenuAppearanceValue,
  type AccountMenuTriggerMode,
} from "@yueli/ui/account-menu/pattern";
import {
  identityAccountMenuAppearanceMessages,
  identityAccountMenuMessages,
} from "../utils/account-menu";
import { accountMediaUrl } from "../utils/account-media";

const props = withDefaults(
  defineProps<{
    homeTo?: string;
    homeLabel?: string;
    loginLabel?: string;
    showAppearance?: boolean;
    triggerMode?: AccountMenuTriggerMode;
  }>(),
  {
    homeTo: "/",
    homeLabel: "返回主站",
    loginLabel: "登录",
    showAppearance: false,
    triggerMode: "inline",
  },
);

const { user, loggedIn, login, logout } = useAuth();
const accountUrl = computed(
  () => useRuntimeConfig().public.accountUrl || "http://localhost:3000",
);
const avatarUrl = computed(() =>
  accountMediaUrl(user.value?.avatar, accountUrl.value),
);
const colorMode = useColorMode();

function appearanceValue(value: string): AccountMenuAppearanceValue {
  return value === "light" || value === "dark" ? value : "system";
}

const appearance = computed<AccountMenuAppearance | undefined>(() =>
  props.showAppearance
    ? {
        value: appearanceValue(colorMode.preference),
        messages: identityAccountMenuAppearanceMessages,
        onChange: (value) => {
          colorMode.preference = value;
        },
      }
    : undefined,
);

const utilityActions = computed<AccountMenuAction[]>(() => {
  const actions: AccountMenuAction[] = [];
  if (props.homeTo) {
    actions.push({
      label: props.homeLabel,
      icon: "i-tabler-arrow-back-up",
      to: props.homeTo,
    });
  }
  if (accountUrl.value && accountUrl.value !== props.homeTo) {
    actions.push({
      label: "用户设置",
      icon: "i-tabler-user-cog",
      onSelect: async () => {
        await navigateTo(accountUrl.value, { external: true });
      },
    });
  }
  return actions;
});

async function handleLogin(): Promise<void> {
  await login();
}
</script>

<template>
  <AccountMenu
    v-if="loggedIn"
    :name="user?.name"
    :email="user?.email"
    :avatar-url="avatarUrl"
    :utility-actions
    :appearance
    :trigger-mode="props.triggerMode"
    :logout
    :messages="identityAccountMenuMessages"
  />
  <UButton
    v-else
    color="neutral"
    variant="ghost"
    icon="i-tabler-login-2"
    :label="props.triggerMode === 'collapsed' ? undefined : props.loginLabel"
    :block="props.triggerMode === 'sidebar'"
    :square="props.triggerMode === 'collapsed'"
    :class="[
      props.triggerMode === 'sidebar' && 'w-full justify-start',
      props.triggerMode === 'collapsed' && 'aspect-square',
    ]"
    @click="handleLogin"
  />
</template>
