import { computed, defineComponent, ref } from "vue";
import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ConsumerManageAccountControl from "../app/components/ConsumerManageAccountControl.vue";

const AccountMenu = defineComponent({
  name: "AccountMenu",
  props: {
    name: String,
    email: String,
    avatarUrl: String,
    utilityActions: Array,
    appearance: Object,
    triggerMode: String,
    logout: Function,
  },
  template: '<div data-manage-account-menu="authenticated" />',
});

const UButton = defineComponent({
  name: "UButton",
  inheritAttrs: false,
  props: { label: String },
  emits: ["click"],
  template: '<button v-bind="$attrs" @click="$emit(\'click\')">{{ label }}</button>',
});

const user = ref<{
  sub: string;
  name?: string;
  email?: string;
  avatar?: string;
} | null>(null);
const login = vi.fn();
const logout = vi.fn();
const navigateTo = vi.fn();

describe("ConsumerManageAccountControl", () => {
  beforeEach(() => {
    user.value = null;
    login.mockReset();
    logout.mockReset();
    navigateTo.mockReset();
    vi.stubGlobal("useAuth", () => ({
      user,
      loggedIn: computed(() => Boolean(user.value)),
      login,
      logout,
    }));
    vi.stubGlobal("useRuntimeConfig", () => ({
      public: { accountUrl: "https://account.example" },
    }));
    vi.stubGlobal("useColorMode", () => ({ preference: "system" }));
    vi.stubGlobal("navigateTo", navigateTo);
  });

  it("delegates authenticated management identity to the restricted menu", () => {
    user.value = {
      sub: "user-1",
      name: "月离",
      email: "user@example.com",
      avatar: "https://identity.example/avatar.png",
    };

    const wrapper = mount(ConsumerManageAccountControl, {
      props: {
        homeTo: "",
        showAppearance: true,
        triggerMode: "sidebar",
      },
      global: { stubs: { AccountMenu, UButton } },
    });

    const menu = wrapper.getComponent(AccountMenu);
    expect(menu.props()).toMatchObject({
      name: "月离",
      email: "user@example.com",
      avatarUrl: "https://identity.example/avatar.png",
      triggerMode: "sidebar",
    });
    expect(
      menu.props("utilityActions").map((item: { label: string }) => item.label),
    ).toEqual(["用户设置"]);
    expect(menu.props("appearance")).toMatchObject({ value: "system" });
  });

  it("uses the shared login action for anonymous operators", async () => {
    const wrapper = mount(ConsumerManageAccountControl, {
      global: { stubs: { AccountMenu, UButton } },
    });

    await wrapper.get("button").trigger("click");

    expect(wrapper.get("button").text()).toBe("登录");
    expect(login).toHaveBeenCalledOnce();
  });
});
