<script setup lang="ts">
import * as z from "zod";
import type { FormSubmitEvent } from "@nuxt/ui";
import { PageHeader } from "@yueli/ui/admin";
import { SOCIAL_PLATFORMS, socialPlatform } from "~/utils/social";
import { createAccountNotifier } from "~/utils/feedback";
import { useActionFeedback } from "@yueli/ui/feedback";
import { ActionFeedbackButton } from "@yueli/ui/feedback/pattern";
import type { SocialLink } from "~/composables/useSession";

definePageMeta({ middleware: "auth", layout: "account" });
useSeoMeta({ title: "个人资料" });

const { me, refresh } = useSession();
const { call } = useApi();
const toast = createAccountNotifier(useToast());

const initial = computed(() =>
  (me.value?.displayName || me.value?.email || "?").charAt(0).toUpperCase(),
);
const publicProfileTo = computed(() =>
  me.value?.handle ? `/@${me.value.handle}` : me.value?.userKey ? `/u/${me.value.userKey}` : "",
);

const resending = ref(false);
async function onResendVerification() {
  resending.value = true;
  try {
    await call("/api/v1/auth/email/verify-request", { method: "POST" });
    toast.add({
      title: "验证邮件已发送",
      description: "请检查邮箱并完成验证。",
      color: "success",
      icon: "i-tabler-mail-check",
    });
  } catch (error: any) {
    toast.add({
      title: "发送失败",
      description: identityErrorMessage(error, {
        context: "verification",
        fallback: "暂时无法发送验证邮件。",
      }),
      color: "error",
    });
  } finally {
    resending.value = false;
  }
}

const profileSchema = z.object({
  displayName: z.string().min(1, "请输入昵称"),
  handle: z
    .string()
    .regex(/^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$/, "Handle 需为 3–30 位小写字母、数字或下划线")
    .or(z.literal(""))
    .optional(),
  bio: z.string().max(500, "简介最多 500 字").optional(),
  locale: z.string().optional(),
});
type ProfileSchema = z.output<typeof profileSchema>;
const profileState = reactive<ProfileSchema>({
  displayName: "",
  handle: "",
  bio: "",
  locale: "",
});
const avatarUrl = ref("");
const coverUrl = ref("");

const platformItems = SOCIAL_PLATFORMS.map((platform) => ({
  label: platform.label,
  value: platform.key,
  icon: platform.icon,
}));
const platform = (key: string) =>
  SOCIAL_PLATFORMS.find((item) => item.key === key) ??
  SOCIAL_PLATFORMS[SOCIAL_PLATFORMS.length - 1]!;
interface SocialRow {
  key: string;
  url: string;
}
const socialRows = ref<SocialRow[]>([]);

watchEffect(() => {
  if (!me.value) return;
  profileState.displayName = me.value.displayName;
  profileState.handle = me.value.handle;
  profileState.bio = me.value.bio;
  avatarUrl.value = me.value.avatarUrl;
  coverUrl.value = me.value.coverUrl;
  socialRows.value = (me.value.socialLinks ?? []).map((link) => ({
    key: socialPlatform(link).key,
    url: link.url,
  }));
});

function addLink() {
  const used = new Set(socialRows.value.map((row) => row.key));
  const next = SOCIAL_PLATFORMS.find((item) => !used.has(item.key)) ?? SOCIAL_PLATFORMS[0]!;
  socialRows.value.push({ key: next.key, url: "" });
}

function removeLink(index: number) {
  socialRows.value.splice(index, 1);
}

const {
  status: profileSaveStatus,
  pending: markProfileSaving,
  success: markProfileSaved,
  error: markProfileError,
} = useActionFeedback();

async function onSaveProfile(event: FormSubmitEvent<ProfileSchema>) {
  markProfileSaving();
  try {
    const socialLinks: SocialLink[] = socialRows.value
      .filter((row) => row.url.trim())
      .map((row) => ({ label: platform(row.key).label, url: row.url.trim() }));
    await call("/api/v1/session/profile", { method: "PUT", body: event.data });
    await call("/api/v1/session/profile/social-links", {
      method: "PUT",
      body: { socialLinks },
    });
    await refresh();
    markProfileSaved();
  } catch (error: any) {
    markProfileError();
    toast.add({
      title: "保存失败",
      description: identityErrorMessage(error, {
        context: "profile",
        fallback: "暂时无法保存账户资料。",
      }),
      color: "error",
    });
  }
}

async function onAvatarUpdated(url: string) {
  avatarUrl.value = url;
  await refresh();
}

async function onCoverUpdated(url: string) {
  coverUrl.value = url;
  await refresh();
}
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="个人资料" icon="i-tabler-user-edit">
      <template v-if="publicProfileTo" #actions>
        <UButton
          :to="publicProfileTo"
          color="neutral"
          variant="outline"
          icon="i-tabler-external-link"
          label="查看公开主页"
        />
      </template>
    </PageHeader>

    <UAlert
      v-if="me && !me.emailVerified"
      color="warning"
      variant="soft"
      icon="i-tabler-mail-x"
      title="邮箱尚未验证"
      description="验证邮箱以保护账户安全。"
    >
      <template #actions>
        <UButton
          color="warning"
          size="sm"
          label="重新发送验证邮件"
          :loading="resending"
          @click="onResendVerification"
        />
      </template>
    </UAlert>

    <div class="grid min-w-0 gap-5 xl:grid-cols-[minmax(18rem,0.72fr)_minmax(0,1.28fr)]">
      <section class="yueli-card min-w-0 self-start overflow-hidden">
        <UserCoverCrop
          :model-value="coverUrl"
          editable
          @update:model-value="onCoverUpdated"
        />
        <div class="flex items-end gap-4 px-5 pb-5">
          <div class="-mt-12 shrink-0 rounded-full ring-4 ring-default">
            <UserAvatarCrop
              :model-value="avatarUrl"
              :initial="initial"
              editable
              @update:model-value="onAvatarUpdated"
            />
          </div>
          <div class="min-w-0 flex-1 pb-1">
            <p class="flex items-center gap-2 text-lg font-semibold text-highlighted">
              <span class="truncate">{{ me?.displayName || "我的账户" }}</span>
              <UBadge
                v-if="me?.emailVerified"
                color="success"
                variant="subtle"
                size="sm"
                icon="i-tabler-rosette-discount-check"
              >
                已验证
              </UBadge>
            </p>
            <p class="truncate text-sm text-muted">{{ me?.email }}</p>
          </div>
        </div>
      </section>

      <UCard class="min-w-0">
        <template #header>
          <h2 class="font-semibold text-highlighted">资料信息</h2>
        </template>
        <UForm
          :schema="profileSchema"
          :state="profileState"
          class="space-y-5"
          @submit="onSaveProfile"
        >
          <div class="grid gap-4 lg:grid-cols-2">
            <UFormField name="displayName" label="昵称">
              <UInput v-model="profileState.displayName" class="w-full" placeholder="你的昵称" />
            </UFormField>
            <UFormField
              name="handle"
              label="公开主页地址"
            >
              <UInput v-model="profileState.handle" class="w-full" placeholder="yueli">
                <template #leading>
                  <span class="text-muted">@</span>
                </template>
              </UInput>
            </UFormField>
          </div>
          <UFormField name="bio" label="简介" hint="最多 500 字">
            <UTextarea
              v-model="profileState.bio"
              :rows="4"
              autoresize
              :maxrows="8"
              class="w-full"
              placeholder="一句话介绍自己"
            />
          </UFormField>

          <section aria-labelledby="social-links-title">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
              <h3 id="social-links-title" class="text-sm font-medium text-default">社交链接</h3>
              <UButton
                label="添加链接"
                icon="i-tabler-plus"
                color="neutral"
                variant="soft"
                size="xs"
                :disabled="socialRows.length >= platformItems.length"
                @click="addLink"
              />
            </div>
            <div
              v-if="!socialRows.length"
              class="rounded-lg border border-dashed border-default px-4 py-7 text-center"
            >
              <p class="text-sm font-medium text-highlighted">还没有社交链接</p>
              <p class="mt-1 text-xs text-muted">添加后会显示在你的公开主页。</p>
            </div>
            <div v-else class="space-y-3">
              <div
                v-for="(row, index) in socialRows"
                :key="`${row.key}-${index}`"
                class="grid min-w-0 gap-2 sm:grid-cols-[9rem_minmax(0,1fr)_auto]"
              >
                <USelectMenu
                  v-model="row.key"
                  :items="platformItems"
                  value-key="value"
                  :icon="platform(row.key).icon"
                  :search-input="false"
                />
                <UInput
                  v-model="row.url"
                  :placeholder="platform(row.key).placeholder"
                  class="min-w-0"
                />
                <UButton
                  icon="i-tabler-trash"
                  color="error"
                  variant="ghost"
                  square
                  aria-label="删除链接"
                  @click="removeLink(index)"
                />
              </div>
            </div>
          </section>

          <div class="flex justify-end border-t border-default pt-4">
            <ActionFeedbackButton
              type="submit"
              :status="profileSaveStatus"
              idle-label="保存资料"
              pending-label="保存中"
              success-label="已保存"
              error-label="保存失败"
            />
          </div>
        </UForm>
      </UCard>
    </div>
  </div>
</template>
