import type {
  AccountMenuAppearanceMessages,
  AccountMenuMessages,
} from "@yueli/ui/account-menu/pattern";

export const accountMenuMessages: AccountMenuMessages = {
  currentUser: "当前用户",
  logout: "退出登录",
  openMenu: (displayName) => `打开${displayName}的用户菜单`,
};

export const accountMenuAppearanceMessages: AccountMenuAppearanceMessages = {
  label: "外观",
  system: "跟随系统",
  light: "浅色",
  dark: "深色",
};
