<script setup lang="ts">
import * as z from "zod";
import type { FormSubmitEvent } from "@nuxt/ui";
import { PageHeader } from "@yueli/ui/admin";
import { createAccountNotifier } from "~/utils/feedback";
import { useActionFeedback } from "@yueli/ui/feedback";
import { ActionFeedbackButton } from "@yueli/ui/feedback/pattern";

definePageMeta({ middleware: "auth", layout: "account" });
useSeoMeta({ title: "账户安全" });

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

const passwordSchema = z
  .object({
    currentPassword: z.string().min(1, "请输入当前密码"),
    newPassword: newPasswordSchema("新密码"),
    confirm: z.string().min(1, "请再次输入新密码"),
  })
  .refine((data) => data.newPassword === data.confirm, {
    message: "两次输入不一致",
    path: ["confirm"],
  });
type PasswordSchema = z.output<typeof passwordSchema>;
const passwordState = reactive<Partial<PasswordSchema>>({
  currentPassword: "",
  newPassword: "",
  confirm: "",
});
const {
  status: passwordStatus,
  pending: markPasswordSaving,
  success: markPasswordSaved,
  error: markPasswordError,
} = useActionFeedback();

async function onChangePassword(event: FormSubmitEvent<PasswordSchema>) {
  markPasswordSaving();
  try {
    await call("/api/v1/auth/password/change", {
      method: "POST",
      body: {
        currentPassword: event.data.currentPassword,
        newPassword: event.data.newPassword,
      },
    });
    passwordState.currentPassword = "";
    passwordState.newPassword = "";
    passwordState.confirm = "";
    markPasswordSaved();
  } catch (error: any) {
    markPasswordError();
    toast.add({
      title: "修改失败",
      description: identityErrorMessage(error, {
        context: "password-change",
        fallback: "暂时无法修改密码。",
      }),
      color: "error",
    });
  }
}

const initialPasswordSchema = z
  .object({
    newPassword: newPasswordSchema(),
    confirm: z.string().min(1, "请再次输入密码"),
  })
  .refine((data) => data.newPassword === data.confirm, {
    message: "两次输入不一致",
    path: ["confirm"],
  });
type InitialPasswordSchema = z.output<typeof initialPasswordSchema>;
const initialPasswordState = reactive<Partial<InitialPasswordSchema>>({
  newPassword: "",
  confirm: "",
});
const {
  status: initialPasswordStatus,
  pending: markInitialPasswordSaving,
  success: markInitialPasswordSaved,
  error: markInitialPasswordError,
} = useActionFeedback();

async function onSetPassword(event: FormSubmitEvent<InitialPasswordSchema>) {
  markInitialPasswordSaving();
  try {
    await call("/api/v1/auth/password/set", {
      method: "POST",
      body: { newPassword: event.data.newPassword },
    });
    initialPasswordState.newPassword = "";
    initialPasswordState.confirm = "";
    await refreshCredentials();
    markInitialPasswordSaved();
  } catch (error: any) {
    markInitialPasswordError();
    toast.add({
      title: "设置失败",
      description: identityErrorMessage(error, {
        context: "password-set",
        fallback: "暂时无法设置密码。",
      }),
      color: "error",
    });
  }
}
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="账户安全" icon="i-tabler-shield-lock" />

    <UCard>
      <template #header>
        <h2 class="flex items-center gap-2 font-semibold text-highlighted">
          <UIcon
            :name="credentials?.hasPassword ? 'i-tabler-lock' : 'i-tabler-lock-plus'"
            class="size-5 text-primary"
          />
          {{ credentials?.hasPassword ? "修改密码" : "设置密码" }}
        </h2>
      </template>

      <UForm
        v-if="credentials?.hasPassword"
        :schema="passwordSchema"
        :state="passwordState"
        class="space-y-4"
        @submit="onChangePassword"
      >
        <div class="grid gap-4 lg:grid-cols-3">
          <UFormField name="currentPassword" label="当前密码">
            <UInput
              v-model="passwordState.currentPassword"
              type="password"
              autocomplete="current-password"
              class="w-full"
            />
          </UFormField>
          <UFormField name="newPassword" label="新密码">
            <UInput
              v-model="passwordState.newPassword"
              type="password"
              autocomplete="new-password"
              class="w-full"
            />
          </UFormField>
          <UFormField name="confirm" label="确认新密码">
            <UInput
              v-model="passwordState.confirm"
              type="password"
              autocomplete="new-password"
              class="w-full"
            />
          </UFormField>
          <p class="text-xs text-muted lg:col-span-3">{{ PASSWORD_HINT }}</p>
        </div>
        <div class="flex justify-end border-t border-default pt-4">
          <ActionFeedbackButton
            type="submit"
            color="neutral"
            :status="passwordStatus"
            idle-label="修改密码"
            pending-label="修改中"
            success-label="已修改"
            error-label="修改失败"
          />
        </div>
      </UForm>

      <UForm
        v-else
        :schema="initialPasswordSchema"
        :state="initialPasswordState"
        class="space-y-4"
        @submit="onSetPassword"
      >
        <UAlert
          color="info"
          variant="soft"
          icon="i-tabler-info-circle"
          title="当前账户没有密码"
          description="设置后即可使用邮箱和密码登录。"
        />
        <div class="grid gap-4 lg:grid-cols-2">
          <UFormField name="newPassword" label="新密码">
            <UInput
              v-model="initialPasswordState.newPassword"
              type="password"
              autocomplete="new-password"
              class="w-full"
            />
          </UFormField>
          <UFormField name="confirm" label="确认密码">
            <UInput
              v-model="initialPasswordState.confirm"
              type="password"
              autocomplete="new-password"
              class="w-full"
            />
          </UFormField>
          <p class="text-xs text-muted lg:col-span-2">{{ PASSWORD_HINT }}</p>
        </div>
        <div class="flex justify-end border-t border-default pt-4">
          <ActionFeedbackButton
            type="submit"
            :status="initialPasswordStatus"
            idle-label="设置密码"
            pending-label="设置中"
            success-label="已设置"
            error-label="设置失败"
          />
        </div>
      </UForm>
    </UCard>

    <div class="grid min-w-0 gap-5 xl:grid-cols-2 [&>*]:min-w-0 [&>*]:self-start">
      <PasskeyManager @changed="refreshCredentials" />
      <TOTPManager />
    </div>
  </div>
</template>
