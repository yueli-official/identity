<script setup lang="ts">
import type { PublicUser } from "~/types/public-user";
import { socialPlatform } from "~/utils/social";

const props = defineProps<{
  user: PublicUser;
}>();

const initial = computed(() =>
  (props.user.displayName || props.user.handle || "?").charAt(0).toUpperCase(),
);
const avatarUrl = computed(() => userMediaUrl(props.user.avatar, "thumbnail"));
const coverUrl = computed(() => userMediaUrl(props.user.cover, "cover"));
const stableProfilePath = computed(
  () => `/u/${encodeURIComponent(props.user.userKey)}`,
);
const links = computed(() =>
  (props.user.socialLinks || []).map((link) => ({
    ...link,
    platform: socialPlatform(link),
  })),
);
</script>

<template>
  <!--
  THESIS: 公开资料是一张可验证的身份凭证，不是社交数据面板；可修改 Handle 与永久用户号必须一眼分层。
  OWN-WORLD: 沿用 Account 的 teal、中性色、柔和圆角与封面—头像语言；身份正文克制，永久键使用数据字体。
  STORY: 访客先确认姓名与 Handle，再读简介，以永久用户号确认稳定身份，最后访问用户主动公开的链接。
  FIRST VIEWPORT: 封面横贯顶部，头像压住封面边缘；姓名与 Handle 在左，永久用户号在右，简介紧随其后。
  FORM: 七个既有视觉世界结构中的第四案“公开身份凭证”，采用稳定键侧栏；seed 341b984d。
  FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
  -->
  <article class="mx-auto w-full max-w-2xl pb-6">
    <section
      class="overflow-hidden rounded-2xl bg-default ring-1 ring-default"
      aria-labelledby="profile-name"
    >
      <div class="relative h-36 overflow-hidden bg-primary/10 sm:h-48">
        <img
          v-if="coverUrl"
          :src="coverUrl"
          alt=""
          class="size-full object-cover"
        >
      </div>

      <div class="px-5 pb-6 sm:px-8 sm:pb-8">
        <div class="-mt-12 w-fit rounded-full bg-default p-1 sm:-mt-14">
          <UAvatar
            :src="avatarUrl || undefined"
            :text="initial"
            :alt="`${user.displayName}的头像`"
            size="3xl"
            class="size-24 shadow-soft sm:size-28"
          />
        </div>

        <div class="mt-4 flex flex-col gap-6 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0 flex-1">
            <h1
              id="profile-name"
              class="font-display text-balance text-2xl font-semibold text-highlighted sm:text-3xl"
            >
              {{ user.displayName }}
            </h1>
            <p v-if="user.handle" class="mt-1 break-all text-base text-muted">
              @{{ user.handle }}
            </p>
          </div>

          <div class="min-w-0 rounded-xl bg-elevated px-4 py-3 sm:w-52 sm:shrink-0">
            <p class="text-xs font-medium text-muted">永久用户号</p>
            <NuxtLink
              :to="stableProfilePath"
              class="mt-1 flex min-h-7 items-center gap-2 rounded-sm font-mono text-sm font-semibold text-highlighted outline-none hover:text-primary focus-visible:ring-2 focus-visible:ring-primary"
              :aria-label="`打开 ${user.displayName} 的永久资料地址`"
            >
              <UIcon name="i-tabler-link" class="size-4 shrink-0 text-primary" />
              <span class="min-w-0 break-all">{{ user.userKey }}</span>
            </NuxtLink>
          </div>
        </div>

        <p
          v-if="user.bio"
          class="mt-6 max-w-[68ch] whitespace-pre-line text-pretty text-base leading-7 text-default"
        >
          {{ user.bio }}
        </p>
      </div>
    </section>

    <section v-if="links.length" class="mt-8" aria-labelledby="public-links-heading">
      <h2
        id="public-links-heading"
        class="font-display text-lg font-semibold text-highlighted"
      >
        公开链接
      </h2>
      <ul class="mt-3 grid gap-3 sm:grid-cols-2">
        <li v-for="link in links" :key="`${link.label}:${link.url}`">
          <UButton
            :to="link.url"
            external
            target="_blank"
            rel="noopener noreferrer"
            color="neutral"
            variant="outline"
            size="lg"
            block
            :icon="link.platform.icon"
            trailing-icon="i-tabler-arrow-up-right"
            :label="link.label"
            class="min-h-12 justify-between"
          />
        </li>
      </ul>
    </section>

    <p class="mt-8 flex items-center justify-center gap-2 text-center text-xs text-muted">
      <UIcon name="i-tabler-shield-check" class="size-4 text-primary" />
      资料由月离账户中心统一提供
    </p>
  </article>
</template>
