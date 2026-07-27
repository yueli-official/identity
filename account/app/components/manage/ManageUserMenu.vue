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
  accountMenuAppearanceMessages,
  accountMenuMessages,
} from "~/utils/account-menu";

const props = withDefaults(
  defineProps<{
    name?: string;
    email?: string;
    avatarUrl?: string;
    homeTo?: string;
    homeLabel?: string;
    settingsTo?: string;
    settingsLabel?: string;
    settingsExternal?: boolean;
    showAppearance?: boolean;
    triggerMode?: AccountMenuTriggerMode;
    logout: () => unknown | Promise<unknown>;
  }>(),
  {
    name: "",
    email: "",
    avatarUrl: "",
    homeTo: "/",
    homeLabel: "返回主站",
    settingsTo: "",
    settingsLabel: "用户设置",
    settingsExternal: true,
    showAppearance: false,
    triggerMode: "inline",
  },
);

const colorMode = useColorMode();

function appearanceValue(value: string): AccountMenuAppearanceValue {
  return value === "light" || value === "dark" ? value : "system";
}

const appearance = computed<AccountMenuAppearance | undefined>(() =>
  props.showAppearance
    ? {
        value: appearanceValue(colorMode.preference),
        messages: accountMenuAppearanceMessages,
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
  if (props.settingsTo && props.settingsTo !== props.homeTo) {
    actions.push({
      label: props.settingsLabel,
      icon: "i-tabler-user-cog",
      onSelect: async () => {
        await navigateTo(props.settingsTo, {
          external: props.settingsExternal,
        });
      },
    });
  }
  return actions;
});
</script>

<template>
  <AccountMenu
    :name
    :email
    :avatar-url="avatarUrl"
    :utility-actions="utilityActions"
    :appearance
    :trigger-mode="triggerMode"
    :logout
    :messages="accountMenuMessages"
  />
</template>
