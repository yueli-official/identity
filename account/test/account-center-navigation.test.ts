import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const root = resolve(import.meta.dirname, "..");
const read = (path: string) => readFileSync(resolve(root, path), "utf8");
const layout = read("app/layouts/account.vue");
const profile = read("app/pages/index.vue");
const security = read("app/pages/security.vue");
const connections = read("app/pages/connections.vue");
const sessions = read("app/pages/sessions.vue");
const developerTokens = read("app/pages/developer-tokens.vue");
const adminLayout = read("app/layouts/admin.vue");
const providerAdmin = read("app/pages/admin/login-providers.vue");

describe("Account 账户中心信息架构", () => {
  it("复用共享管理壳并给四项账户任务稳定路由", () => {
    expect(layout).toContain("<YAdminConsoleLayout");
    expect(layout).toContain('brand-label="账户中心"');
    expect(layout).toContain('to: "/"');
    expect(layout).toContain('to: "/security"');
    expect(layout).toContain('to: "/connections"');
    expect(layout).toContain('to: "/sessions"');
    expect(layout).toContain('to: "/developer-tokens"');
    expect(layout).toContain("data-account-center-shell");
  });

  it("各页面只承担自己的任务而不是重新拼回单页", () => {
    for (const page of [profile, security, connections, sessions, developerTokens]) {
      expect(page).toContain('layout: "account"');
      expect(page).toContain("<PageHeader");
    }
    expect(profile).not.toContain("修改密码");
    expect(profile).not.toContain("登录会话");
    expect(security).not.toContain("社交链接");
    expect(connections).not.toContain("登录会话");
  });

  it("把开发者令牌和登录提供商放在各自控制面", () => {
    expect(developerTokens).toContain("/api/v1/pat/scopes");
    expect(developerTokens).toContain("令牌只显示这一次");
    expect(adminLayout).toContain('to: "/admin/login-providers"');
    expect(providerAdmin).toContain("X-Step-Up-Proof");
    expect(providerAdmin).not.toContain("clientSecretCipher");
  });

  it("会话页分页读取并保留当前会话的批量退出语义", () => {
    expect(sessions).toContain("limit=${pageSize}&offset=");
    expect(sessions).toContain("/api/v1/auth/logout-others");
    expect(sessions).toContain("退出其他会话");
    expect(sessions).not.toContain("其他设备");
    expect(sessions).not.toContain("/api/v1/auth/logout-all");
  });
});
