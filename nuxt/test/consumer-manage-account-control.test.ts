import { computed, defineComponent, ref } from "vue";
import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ConsumerManageAccountControl from "../app/components/ConsumerManageAccountControl.vue";

const ManageUserMenu = defineComponent({
  name: "ManageUserMenu",
  props: {
    name: String,
    email: String,
    avatarUrl: String,
    homeTo: String,
    settingsTo: String,
    showAppearance: Boolean,
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

describe("ConsumerManageAccountControl", () => {
  beforeEach(() => {
    user.value = null;
    login.mockReset();
    logout.mockReset();
    vi.stubGlobal("useAuth", () => ({
      user,
      loggedIn: computed(() => Boolean(user.value)),
      login,
      logout,
    }));
    vi.stubGlobal("useRuntimeConfig", () => ({
      public: { accountUrl: "https://account.example" },
    }));
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
      global: { stubs: { ManageUserMenu, UButton } },
    });

    expect(wrapper.getComponent(ManageUserMenu).props()).toMatchObject({
      name: "月离",
      email: "user@example.com",
      avatarUrl: "https://identity.example/avatar.png",
      homeTo: "",
      settingsTo: "https://account.example",
      showAppearance: true,
      triggerMode: "sidebar",
    });
  });

  it("uses the shared login action for anonymous operators", async () => {
    const wrapper = mount(ConsumerManageAccountControl, {
      global: { stubs: { ManageUserMenu, UButton } },
    });

    await wrapper.get("button").trigger("click");

    expect(wrapper.get("button").text()).toBe("登录");
    expect(login).toHaveBeenCalledOnce();
  });
});
