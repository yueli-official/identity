<script setup lang="ts">
import { computed } from "vue";
import { ManageUserMenu } from "@platform/manage/components";

const props = withDefaults(
  defineProps<{
    homeTo?: string;
    homeLabel?: string;
    loginLabel?: string;
  }>(),
  {
    homeTo: "/",
    homeLabel: "返回主站",
    loginLabel: "登录",
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
    :logout
  />
  <UButton
    v-else
    color="neutral"
    variant="ghost"
    icon="i-tabler-login-2"
    :label="props.loginLabel"
    @click="handleLogin"
  />
</template>
