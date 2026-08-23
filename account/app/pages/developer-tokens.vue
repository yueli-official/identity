<script setup lang="ts">
import * as z from "zod";
import type { FormSubmitEvent } from "@nuxt/ui";
import { PageHeader } from "@yueli/ui/admin";
import { createAccountNotifier } from "~/utils/feedback";

definePageMeta({ middleware: "auth", layout: "account" });
useSeoMeta({ title: "开发者令牌" });

interface TokenEntry {
  id: number;
  name: string;
  tokenPrefix: string;
  scopes: string[];
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
}
interface ScopeEntry {
  key: string;
  label: string;
  description: string;
}
interface CreatedToken extends TokenEntry {
  token: string;
}

const { call } = useApi();
const toast = createAccountNotifier(useToast());
const { data, pending, error, refresh } = await useAsyncData(
  "personal-access-tokens",
  () => call<{ entries: TokenEntry[] }>("/api/v1/pat"),
  { default: () => ({ entries: [] }) },
);
const { data: scopeData } = await useAsyncData(
  "personal-access-token-scopes",
  () => call<{ entries: ScopeEntry[] }>("/api/v1/pat/scopes"),
  { default: () => ({ entries: [] }) },
);
const scopeItems = computed(() =>
  scopeData.value.entries.map((scope) => ({
    label: scope.label,
    value: scope.key,
    description: scope.description,
  })),
);

const schema = z.object({
  name: z.string().min(1, "请输入令牌名称").max(100, "名称最多 100 个字符"),
  scopes: z.array(z.string()).min(1, "至少选择一项权限"),
  expiresInDays: z.number().int().min(0),
});
type TokenForm = z.output<typeof schema>;
const state = reactive<TokenForm>({ name: "", scopes: [], expiresInDays: 90 });
const expiryOptions = [
  { label: "30 天", value: 30 },
  { label: "90 天", value: 90 },
  { label: "一年", value: 365 },
  { label: "永不过期", value: 0 },
];
const modalOpen = ref(false);
const creating = ref(false);
const createError = ref("");
const created = ref<CreatedToken>();
const copied = ref(false);
const revoking = ref<number>();

function openCreate() {
  state.name = "";
  state.scopes = [];
  state.expiresInDays = 90;
  created.value = undefined;
  copied.value = false;
  createError.value = "";
  modalOpen.value = true;
}

function closeTokenModal() {
  modalOpen.value = false;
}

async function createToken(event: FormSubmitEvent<TokenForm>) {
  creating.value = true;
  createError.value = "";
  try {
    created.value = await call<CreatedToken>("/api/v1/pat", {
      method: "POST",
      body: event.data,
    });
    await refresh();
  } catch (failure) {
    createError.value = identityErrorMessage(failure, {
      fallback: "暂时无法创建开发者令牌。",
    });
  } finally {
    creating.value = false;
  }
}

async function copyToken() {
  if (!created.value) return;
  await navigator.clipboard.writeText(created.value.token);
  copied.value = true;
}

async function revokeToken(token: TokenEntry) {
  revoking.value = token.id;
  try {
    await call(`/api/v1/pat/${token.id}`, { method: "DELETE" });
    await refresh();
  } catch (failure) {
    toast.add({
      title: "撤销失败",
      description: identityErrorMessage(failure, { fallback: "暂时无法撤销该令牌。" }),
      color: "error",
    });
  } finally {
    revoking.value = undefined;
  }
}

function formatDate(value?: string) {
  if (!value) return "从未";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

function scopeLabel(key: string) {
  return scopeData.value.entries.find((scope) => scope.key === key)?.label || key;
}
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="开发者令牌" icon="i-tabler-api">
      <template #actions>
        <UButton icon="i-tabler-plus" label="创建令牌" @click="openCreate" />
      </template>
    </PageHeader>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      title="开发者令牌加载失败"
      :description="pageErrorMessage(error)"
    />

    <UCard v-else>
      <div v-if="pending" class="space-y-3" aria-label="正在加载开发者令牌">
        <USkeleton v-for="item in 3" :key="item" class="h-20 w-full rounded-lg" />
      </div>
      <div
        v-else-if="!data.entries.length"
        class="rounded-lg border border-dashed border-default px-4 py-10 text-center"
      >
        <span class="mx-auto grid size-10 place-items-center rounded-lg bg-elevated text-muted">
          <UIcon name="i-tabler-api-app-off" class="size-5" />
        </span>
        <p class="mt-3 text-sm font-medium text-highlighted">还没有开发者令牌</p>
      </div>
      <ul v-else class="divide-y divide-default">
        <li
          v-for="token in data.entries"
          :key="token.id"
          class="flex flex-wrap items-center gap-4 py-4 first:pt-0 last:pb-0"
        >
          <span class="grid size-10 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
            <UIcon name="i-tabler-key" class="size-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <p class="font-medium text-highlighted">{{ token.name }}</p>
              <code class="rounded bg-elevated px-1.5 py-0.5 text-xs text-muted">
                {{ token.tokenPrefix }}
              </code>
            </div>
            <p class="mt-1 text-sm text-muted">
              最近使用 {{ formatDate(token.lastUsedAt) }} ·
              {{ token.expiresAt ? `到期 ${formatDate(token.expiresAt)}` : "永不过期" }}
            </p>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <UBadge v-for="scope in token.scopes" :key="scope" color="neutral" variant="soft">
                {{ scopeLabel(scope) }}
              </UBadge>
            </div>
          </div>
          <UButton
            color="error"
            variant="ghost"
            icon="i-tabler-trash"
            label="撤销"
            :loading="revoking === token.id"
            @click="revokeToken(token)"
          />
        </li>
      </ul>
    </UCard>

    <UModal
      v-model:open="modalOpen"
      :dismissible="!created"
      :title="created ? '保存开发者令牌' : '创建开发者令牌'"
    >
      <template #body>
        <div v-if="created" class="space-y-4">
          <UAlert
            color="warning"
            variant="soft"
            icon="i-tabler-alert-triangle"
            title="令牌只显示这一次"
            description="离开后无法再次查看，请立即保存到密码管理器。"
          />
          <UFieldGroup class="w-full">
            <UInput :model-value="created.token" readonly class="min-w-0 flex-1 font-mono" />
            <UButton
              color="neutral"
              variant="outline"
              :icon="copied ? 'i-tabler-check' : 'i-tabler-copy'"
              :label="copied ? '已复制' : '复制'"
              @click="copyToken"
            />
          </UFieldGroup>
          <div class="flex justify-end">
            <UButton label="我已保存" @click="closeTokenModal" />
          </div>
        </div>

        <UForm
          v-else
          id="create-personal-access-token"
          :schema="schema"
          :state="state"
          class="space-y-4"
          @submit="createToken"
        >
          <UFormField name="name" label="名称" required>
            <UInput v-model="state.name" class="w-full" placeholder="例如：本地脚本" />
          </UFormField>
          <UFormField name="expiresInDays" label="有效期">
            <USelect
              v-model="state.expiresInDays"
              :items="expiryOptions"
              value-key="value"
              class="w-full"
            />
          </UFormField>
          <UFormField name="scopes" label="权限" required>
            <UCheckboxGroup
              v-model="state.scopes"
              :items="scopeItems"
              value-key="value"
              class="space-y-2"
            />
          </UFormField>
          <UAlert v-if="createError" color="error" variant="soft" :title="createError" />
        </UForm>
      </template>
      <template v-if="!created" #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="creating"
            @click="closeTokenModal"
          />
          <UButton
            type="submit"
            form="create-personal-access-token"
            label="创建令牌"
            :loading="creating"
          />
        </div>
      </template>
    </UModal>
  </div>
</template>
