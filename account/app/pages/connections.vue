<script setup lang="ts">
import { PageHeader } from "@yueli/ui/admin";
import { createAccountNotifier } from "~/utils/feedback";
import { externalLoginProviderMeta } from "~/utils/external-login";

definePageMeta({ middleware: "auth", layout: "account" });
useSeoMeta({ title: "登录方式" });

interface CredentialSummary {
  hasPassword: boolean;
  oauth: { provider: string; email: string }[];
  passkeyCount: number;
}

const { call } = useApi();
const toast = createAccountNotifier(useToast());
const { data: credentials, refresh: refreshCredentials } = await useAsyncData(
  "account-credentials",
  () => call<CredentialSummary>("/api/v1/session/credentials"),
);
const { data: externalProviderData } = await useAsyncData(
  "external-login-providers",
  () => call<{ entries: ExternalLoginProvider[] }>("/api/v1/auth/oauth/providers"),
  { default: () => ({ entries: [] }) },
);
const boundProviderKeys = computed(() =>
  new Set((credentials.value?.oauth ?? []).map((item) => item.provider)),
);
const providers = computed<ExternalLoginProvider[]>(() => {
  const byKey = new Map(
    externalProviderData.value.entries.map((provider) => [provider.key, provider]),
  );
  for (const key of boundProviderKeys.value) {
    if (!byKey.has(key)) {
      byKey.set(key, {
        key,
        label: externalLoginProviderMeta(key).label,
        registrationPolicy: "existing_user_only",
        enabled: false,
      });
    }
  }
  return [...byKey.values()].sort((left, right) => left.label.localeCompare(right.label));
});
const isExternalCredentialLast = computed(
  () =>
    !credentials.value?.hasPassword &&
    (credentials.value?.oauth?.length ?? 0) <= 1 &&
    (credentials.value?.passkeyCount ?? 0) === 0,
);
const providerBound = (key: string) => boundProviderKeys.value.has(key);
const providerBindURL = (key: string) =>
  `/api/v1/auth/oauth/${key}/start?intent=bind&return_to=${encodeURIComponent("/connections")}`;

const confirmUnbindOpen = ref(false);
const unbinding = ref(false);
const selectedProviderKey = ref("");
function openUnbind(key: string) {
  selectedProviderKey.value = key;
  confirmUnbindOpen.value = true;
}

function closeUnbind() {
  confirmUnbindOpen.value = false;
}

async function onUnbindProvider() {
  unbinding.value = true;
  try {
    await call(`/api/v1/session/credentials/${selectedProviderKey.value}`, { method: "DELETE" });
    await refreshCredentials();
    confirmUnbindOpen.value = false;
  } catch (error: any) {
    toast.add({
      title: "解绑失败",
      description: identityErrorMessage(error, {
        context: "credential",
        fallback: "暂时无法移除该登录方式。",
      }),
      color: "error",
    });
  } finally {
    unbinding.value = false;
  }
}

const route = useRoute();
onMounted(() => {
  const message = oauthRedirectErrorMessage(route.query.error, "bind", route.query.provider);
  if (message) {
    toast.add({ title: "账户绑定失败", description: message, color: "error" });
  }
});
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="登录方式" icon="i-tabler-key" />

    <UCard>
      <ul class="divide-y divide-default">
        <li class="flex flex-wrap items-center gap-4 py-4 first:pt-0">
          <span class="grid size-10 shrink-0 place-items-center rounded-lg bg-elevated">
            <UIcon name="i-tabler-lock" class="size-5 text-muted" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="font-medium text-highlighted">密码</p>
            <p class="mt-0.5 text-sm text-muted">使用邮箱和密码登录。</p>
          </div>
          <div class="flex items-center gap-2">
            <UBadge :color="credentials?.hasPassword ? 'success' : 'neutral'" variant="soft">
              {{ credentials?.hasPassword ? "已设置" : "未设置" }}
            </UBadge>
            <UButton to="/security" color="neutral" variant="ghost" label="管理" />
          </div>
        </li>

        <li
          v-for="provider in providers"
          :key="provider.key"
          class="flex flex-wrap items-center gap-4 py-4"
        >
          <span class="grid size-10 shrink-0 place-items-center rounded-lg bg-elevated">
            <UIcon :name="externalLoginProviderMeta(provider.key).icon" class="size-5 text-muted" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="font-medium text-highlighted">{{ provider.label }}</p>
            <p class="mt-0.5 text-sm text-muted">
              {{
                providerBound(provider.key)
                  ? `已绑定，可以使用 ${provider.label} 登录。`
                  : provider.enabled === false
                    ? "登录提供商已停用。"
                    : "尚未绑定。"
              }}
            </p>
          </div>
          <UButton
            v-if="providerBound(provider.key)"
            color="neutral"
            variant="outline"
            label="解绑"
            :disabled="isExternalCredentialLast || unbinding"
            @click="openUnbind(provider.key)"
          />
          <UButton
            v-else
            color="neutral"
            variant="outline"
            :icon="externalLoginProviderMeta(provider.key).icon"
            label="绑定"
            :to="providerBindURL(provider.key)"
            :disabled="provider.enabled === false"
            external
          />
        </li>

        <li class="flex flex-wrap items-center gap-4 py-4 last:pb-0">
          <span class="grid size-10 shrink-0 place-items-center rounded-lg bg-elevated">
            <UIcon name="i-tabler-key" class="size-5 text-muted" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="font-medium text-highlighted">通行密钥</p>
            <p class="mt-0.5 text-sm text-muted">使用指纹、面容或设备 PIN 登录。</p>
          </div>
          <div class="flex items-center gap-2">
            <UBadge :color="credentials?.passkeyCount ? 'success' : 'neutral'" variant="soft">
              {{ credentials?.passkeyCount ? `${credentials.passkeyCount} 个` : "未设置" }}
            </UBadge>
            <UButton to="/security" color="neutral" variant="ghost" label="管理" />
          </div>
        </li>
      </ul>

      <UAlert
        v-if="(credentials?.oauth?.length ?? 0) === 1 && isExternalCredentialLast"
        class="mt-5"
        color="warning"
        variant="soft"
        icon="i-tabler-alert-triangle"
        title="当前第三方账户是唯一登录方式"
        description="先在账户安全中设置密码或通行密钥，再解绑。"
      />
    </UCard>

    <UModal
      v-model:open="confirmUnbindOpen"
      :title="`解绑 ${externalLoginProviderMeta(selectedProviderKey).label}？`"
    >
      <template #body>
        <p class="text-sm text-muted">
          解绑后将无法再使用 {{ externalLoginProviderMeta(selectedProviderKey).label }} 登录此账户。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="unbinding"
            @click="closeUnbind"
          />
          <UButton color="error" label="确认解绑" :loading="unbinding" @click="onUnbindProvider" />
        </div>
      </template>
    </UModal>
  </div>
</template>
