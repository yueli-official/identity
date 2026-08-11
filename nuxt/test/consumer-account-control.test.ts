import { computed, defineComponent, ref } from "vue";
import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ConsumerAccountControl from "../app/components/ConsumerAccountControl.vue";

const AccountMenu = defineComponent({
  name: "AccountMenu",
  props: {
    name: String,
    email: String,
    avatarUrl: String,
    contextActions: Array,
    utilityActions: Array,
    logout: Function,
    triggerMode: String,
  },
  template: '<div data-account-menu="authenticated" />',
});

const UButton = defineComponent({
  name: "UButton",
  inheritAttrs: false,
  props: { label: String, square: Boolean },
  emits: ["click"],
  template:
    '<button v-bind="$attrs" @click="$emit(\'click\')">{{ label }}</button>',
});

const user = ref<{
  sub: string;
  name?: string;
  email?: string;
  avatar?: string;
} | null>(null);
const admin = ref(false);
const login = vi.fn();
const logout = vi.fn();
const navigateTo = vi.fn();

function mountControl(props: Record<string, unknown> = {}) {
  return mount(ConsumerAccountControl, {
    props,
    global: { stubs: { AccountMenu, UButton } },
  });
}

describe("ConsumerAccountControl", () => {
  beforeEach(() => {
    user.value = null;
    admin.value = false;
    login.mockReset();
    logout.mockReset();
    navigateTo.mockReset();
    vi.stubGlobal("useAuth", () => ({
      user,
      loggedIn: computed(() => Boolean(user.value)),
      isAdmin: admin,
      login,
      logout,
    }));
    vi.stubGlobal("useRuntimeConfig", () => ({
      public: { accountUrl: "https://account.example" },
    }));
    vi.stubGlobal("navigateTo", navigateTo);
  });

  it("renders one shared login action for anonymous consumers", async () => {
    const wrapper = mountControl();

    await wrapper.get("button").trigger("click");

    expect(wrapper.get("button").text()).toBe("登录");
    expect(login).toHaveBeenCalledOnce();
  });

  it("renders a compact labelled login action for collapsed chrome", async () => {
    const wrapper = mountControl({ triggerMode: "collapsed" });
    const button = wrapper.get("button");

    expect(button.text()).toBe("");
    expect(button.attributes("aria-label")).toBe("登录");
    expect(wrapper.getComponent(UButton).props("square")).toBe(true);
  });

  it("owns identity, platform utilities and consumer context actions", () => {
    user.value = {
      sub: "user-1",
      name: "月离",
      email: "user@example.com",
      avatar: "https://identity.example/avatar.png",
    };
    admin.value = true;

    const wrapper = mountControl({
      triggerMode: "collapsed",
      manageTo: "/manage",
      homeTo: "/",
      contextActions: [
        { label: "我的购买", icon: "i-tabler-shopping-bag", to: "/orders" },
      ],
    });
    const menu = wrapper.getComponent(AccountMenu);

    expect(menu.props()).toMatchObject({
      name: "月离",
      email: "user@example.com",
      avatarUrl: "https://identity.example/avatar.png",
      triggerMode: "collapsed",
    });
    expect(menu.props("contextActions")).toEqual([
      { label: "我的购买", icon: "i-tabler-shopping-bag", to: "/orders" },
      { label: "控制台", icon: "i-tabler-layout-dashboard", to: "/manage" },
    ]);
    expect(
      menu.props("utilityActions").map((item: { label: string }) => item.label),
    ).toEqual(["返回主站", "用户设置"]);
  });

  it("does not expose the management entry without operator access", () => {
    user.value = { sub: "user-1", name: "访客" };

    const wrapper = mountControl({ manageTo: "/manage" });

    expect(wrapper.getComponent(AccountMenu).props("contextActions")).toEqual(
      [],
    );
  });
});
