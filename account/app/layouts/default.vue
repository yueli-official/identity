<script setup lang="ts">
import { accountMenuMessages } from "~/utils/account-menu";
import {
  AccountMenu,
  type AccountMenuAction,
} from "@yueli/ui/account-menu/pattern";
import { BackToTop } from "@yueli/ui/navigation/back-to-top";

const { me, logout, isAdmin } = useSession();

const utilityActions = computed<AccountMenuAction[]>(() => [
  ...(isAdmin.value
    ? [
        {
          label: "管理控制台",
          icon: "i-tabler-shield-cog",
          to: "/admin/platform",
        },
      ]
    : []),
]);
const handleLogout = async () => {
  await logout();
  await navigateTo("/login");
};
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-default text-default">
    <header
      class="sticky top-0 z-20 border-b border-default bg-default/75 backdrop-blur"
    >
      <div
        class="mx-auto flex h-16 w-full max-w-3xl items-center justify-between gap-4 px-4"
      >
        <NuxtLink
          to="/"
          class="font-display flex items-center gap-2 text-base font-semibold text-highlighted"
        >
          <span
            class="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary"
          >
            <UIcon name="i-tabler-shield-check" class="size-5" />
          </span>
          账户中心
        </NuxtLink>

        <div class="flex items-center gap-1.5">
          <UColorModeButton />
          <AccountMenu
            v-if="me"
            :name="me.displayName"
            :email="me.email"
            :avatar-url="me.avatarUrl || ''"
            :utility-actions="utilityActions"
            :logout="handleLogout"
            :messages="accountMenuMessages"
          />
        </div>
      </div>
    </header>

    <main
      id="public-main"
      tabindex="-1"
      class="mx-auto w-full max-w-3xl flex-1 px-4 py-8 outline-none sm:py-10"
    >
      <slot />
    </main>
    <BackToTop target-id="public-main" label="返回顶部" />
  </div>
</template>
