<script setup lang="ts">
import * as z from "zod";
import type { FormSubmitEvent } from "@nuxt/ui";
import { PageHeader } from "@yueli/ui/admin";
import { pageErrorMessage } from "~/utils/api-errors";
import { externalLoginProviderMeta } from "~/utils/external-login";

definePageMeta({ layout: "admin", middleware: "admin" });
useSeoMeta({ title: "登录配置" });

interface ProviderView {
  key: string;
  label: string;
  registrationPolicy: "verified_email" | "existing_user_only";
  configured: boolean;
  enabled: boolean;
  clientId: string;
  redirectUrl: string;
  secretVersion: number;
  lastHealthOk?: boolean;
  lastHealthCheckedAt?: string;
  lastHealthError?: string;
  updatedAt?: string;
}

const { call } = useApi();
const adminStepUp = useAdminStepUp();
const { data, pending, error, refresh } = await useAsyncData(
  "admin-external-login-providers",
  () => call<{ entries: ProviderView[] }>("/api/v1/admin/login-providers"),
  { default: () => ({ entries: [] }) },
);

const schema = z.object({
  clientId: z.string().min(1, "请输入客户端标识"),
  clientSecret: z.string().optional(),
  enabled: z.boolean(),
});
type ProviderForm = z.output<typeof schema>;
const state = reactive<ProviderForm>({ clientId: "", clientSecret: "", enabled: false });
const selected = ref<ProviderView>();
const modalOpen = ref(false);
const saving = ref(false);
const saveError = ref("");
const checking = ref("");
const copied = ref(false);

const providerHelp = {
  google: {
    summary: "在 Google Cloud 创建“Web 应用”类型的 OAuth 客户端，然后复制 Client ID 和 Client Secret。",
    links: [
      { label: "打开 Google Cloud 凭据", to: "https://console.cloud.google.com/apis/credentials" },
      { label: "查看 Google 接入文档", to: "https://developers.google.com/identity/protocols/oauth2/web-server" },
    ],
  },
  qq: {
    summary: "在 QQ 互联创建网站应用，然后复制 APP ID 和 APP Key。审核通过后才能用于正式登录。",
    links: [
      { label: "打开 QQ 互联应用管理", to: "https://connect.qq.com/manage.html" },
      { label: "查看腾讯 QQ 登录文档", to: "https://cloud.tencent.com/document/product/1441/62653" },
    ],
  },
} as const;
const selectedHelp = computed(() =>
  providerHelp[selected.value?.key as keyof typeof providerHelp] || providerHelp.google,
);

function registrationLabel(provider: ProviderView) {
  return provider.registrationPolicy === "verified_email"
    ? "验证邮箱后可创建账户"
    : "仅绑定已有账户";
}

function statusColor(provider: ProviderView) {
  if (!provider.configured || !provider.enabled) return "neutral";
  return provider.lastHealthOk === false ? "error" : provider.lastHealthOk ? "success" : "warning";
}

function statusLabel(provider: ProviderView) {
  if (!provider.configured) return "未配置";
  if (!provider.enabled) return "已停用";
  if (provider.lastHealthOk === false) return "检查失败";
  if (provider.lastHealthOk) return "正常";
  return "待检查";
}

function openProvider(provider: ProviderView) {
  selected.value = provider;
  state.clientId = provider.clientId;
  state.clientSecret = "";
  state.enabled = provider.enabled;
  saveError.value = "";
  copied.value = false;
  modalOpen.value = true;
}

async function saveProvider(event: FormSubmitEvent<ProviderForm>) {
  if (!selected.value) return;
  saving.value = true;
  saveError.value = "";
  try {
    const providerKey = selected.value.key;
    await adminStepUp.run(
      "identity.external_login_provider.save",
      `identity:external-login:${providerKey}`,
      (proof) =>
        call(`/api/v1/admin/login-providers/${providerKey}`, {
          method: "PUT",
          body: event.data,
          headers: { "X-Step-Up-Proof": proof },
        }),
    );
    await refresh();
    modalOpen.value = false;
  } catch (failure) {
    saveError.value = identityErrorMessage(failure, {
      fallback: "暂时无法保存登录提供商配置。",
    });
  } finally {
    saving.value = false;
  }
}

async function checkProvider(provider: ProviderView) {
  checking.value = provider.key;
  try {
    await call(`/api/v1/admin/login-providers/${provider.key}/health-check`, {
      method: "POST",
    });
    await refresh();
  } finally {
    checking.value = "";
  }
}

async function copyRedirectURL() {
  if (!selected.value) return;
  await navigator.clipboard.writeText(selected.value.redirectUrl);
  copied.value = true;
}

function closeProviderModal() {
  modalOpen.value = false;
}
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="登录配置" icon="i-tabler-login-2" />

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      title="登录提供商加载失败"
      :description="pageErrorMessage(error)"
    />

    <UCard v-else>
      <div v-if="pending" class="space-y-3" aria-label="正在加载登录提供商">
        <USkeleton v-for="item in 2" :key="item" class="h-20 w-full rounded-lg" />
      </div>
      <ul v-else class="divide-y divide-default">
        <li
          v-for="provider in data.entries"
          :key="provider.key"
          class="flex flex-wrap items-center gap-4 py-4 first:pt-0 last:pb-0"
        >
          <span class="grid size-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
            <UIcon :name="externalLoginProviderMeta(provider.key).icon" class="size-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-semibold text-highlighted">{{ provider.label }}</h2>
              <UBadge :color="statusColor(provider)" variant="soft">
                {{ statusLabel(provider) }}
              </UBadge>
              <UBadge color="neutral" variant="outline">
                {{ registrationLabel(provider) }}
              </UBadge>
            </div>
            <p class="mt-1 truncate text-sm text-muted">
              {{ provider.configured ? provider.clientId : "尚未填写客户端凭据" }}
            </p>
            <p v-if="provider.lastHealthError" class="mt-1 text-xs text-error">
              {{ provider.lastHealthError }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <UButton
              v-if="provider.configured && provider.enabled"
              color="neutral"
              variant="ghost"
              icon="i-tabler-heartbeat"
              label="检查"
              :loading="checking === provider.key"
              @click="checkProvider(provider)"
            />
            <UButton
              color="neutral"
              variant="outline"
              icon="i-tabler-settings"
              :label="provider.configured ? '配置' : '开始配置'"
              @click="openProvider(provider)"
            />
          </div>
        </li>
      </ul>
    </UCard>

    <UModal
      v-model:open="modalOpen"
      :title="`配置 ${selected?.label || ''}`"
      :description="selected ? registrationLabel(selected) : ''"
    >
      <template #body>
        <UForm
          id="external-login-provider-form"
          :schema="schema"
          :state="state"
          class="space-y-4"
          @submit="saveProvider"
        >
          <div class="flex items-center gap-1.5 text-sm font-medium text-default">
            <span>在哪里获取凭据？</span>
            <UPopover>
              <UButton
                type="button"
                color="neutral"
                variant="ghost"
                size="xs"
                square
                class="text-muted hover:text-primary"
                aria-label="查看凭据获取帮助"
              >
                <UIcon name="i-tabler-help" class="size-4 shrink-0" />
              </UButton>
              <template #content>
                <div class="w-80 space-y-3 p-3">
                  <p class="text-sm leading-6 text-muted">{{ selectedHelp.summary }}</p>
                  <div class="space-y-1">
                    <UButton
                      v-for="link in selectedHelp.links"
                      :key="link.to"
                      :to="link.to"
                      external
                      target="_blank"
                      color="neutral"
                      variant="ghost"
                      size="sm"
                      trailing-icon="i-tabler-external-link"
                      :label="link.label"
                      class="w-full justify-between"
                    />
                  </div>
                </div>
              </template>
            </UPopover>
          </div>
          <UFormField
            name="clientId"
            :label="externalLoginProviderMeta(selected?.key || '').clientIdLabel"
            required
          >
            <UInput v-model="state.clientId" class="w-full" autocomplete="off" />
          </UFormField>
          <UFormField
            name="clientSecret"
            :label="externalLoginProviderMeta(selected?.key || '').secretLabel"
            :description="selected?.configured ? '留空将继续使用当前密钥。填写新值会创建新的密钥版本。' : '首次配置必须填写。'"
          >
            <UInput
              v-model="state.clientSecret"
              type="password"
              class="w-full"
              autocomplete="new-password"
              :placeholder="selected?.configured ? `当前为 Secret v${selected.secretVersion}` : ''"
            />
          </UFormField>
          <UFormField label="回调地址">
            <UFieldGroup class="w-full">
              <UInput :model-value="selected?.redirectUrl" readonly class="min-w-0 flex-1" />
              <UButton
                type="button"
                color="neutral"
                variant="outline"
                :icon="copied ? 'i-tabler-check' : 'i-tabler-copy'"
                :label="copied ? '已复制' : '复制'"
                @click="copyRedirectURL"
              />
            </UFieldGroup>
          </UFormField>
          <UFormField name="enabled" label="启用登录">
            <USwitch v-model="state.enabled" />
          </UFormField>
          <UAlert v-if="saveError" color="error" variant="soft" :title="saveError" />
        </UForm>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="saving"
            @click="closeProviderModal"
          />
          <UButton
            type="submit"
            form="external-login-provider-form"
            label="保存"
            :loading="saving"
          />
        </div>
      </template>
    </UModal>
  </div>
</template>
