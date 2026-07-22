<script setup lang="ts">
import { computed } from "vue";
import { ManageUserMenu } from "@platform/manage/components";
import type { AccountMenuTriggerMode } from "@yueli/ui/account-menu/pattern";

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

async function handleLogin(): Promise<void> {
  await login();
}
</script>

<template>
  <ManageUserMenu
    v-if="loggedIn"
    :name="user?.name"
    :email="user?.email"
    :avatar-url="user?.avatar"
    :home-to="props.homeTo"
    :home-label="props.homeLabel"
    :settings-to="accountUrl"
    :show-appearance="props.showAppearance"
    :trigger-mode="props.triggerMode"
    :logout
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
