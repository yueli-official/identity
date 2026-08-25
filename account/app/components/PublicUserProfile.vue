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
  () => "/u/" + encodeURIComponent(props.user.userKey),
);
const links = computed(() =>
  (props.user.socialLinks || []).map((link) => ({
    ...link,
    platform: socialPlatform(link),
  })),
);
</script>

<template>
  <article class="mx-auto w-full max-w-3xl pb-6" data-public-user-profile>
    <section
      class="overflow-hidden rounded-2xl border border-default bg-default"
      aria-labelledby="profile-name"
    >
      <div class="relative h-72 overflow-hidden bg-primary/10 sm:h-80">
        <img
          v-if="coverUrl"
          :src="coverUrl"
          alt=""
          class="absolute inset-0 size-full object-cover"
        />
        <div v-else class="public-profile-cover-fallback absolute inset-0" />
        <div
          class="public-profile-shade absolute inset-0"
          data-public-profile-shade
        />

        <div class="absolute inset-x-0 bottom-0 p-5 sm:p-7">
          <div class="flex min-w-0 items-end gap-4 sm:gap-5">
            <UAvatar
              :src="avatarUrl || undefined"
              :text="initial"
              :alt="user.displayName + '的头像'"
              size="3xl"
              class="size-24 shrink-0 shadow-lg ring-4 ring-white/90 sm:size-28"
            />
            <div class="min-w-0 pb-0.5">
              <h1
                id="profile-name"
                class="font-display text-balance text-2xl font-semibold text-white sm:text-3xl"
              >
                {{ user.displayName }}
              </h1>
              <p
                v-if="user.handle"
                class="public-profile-handle mt-0.5 break-all text-sm sm:text-base"
              >
                @{{ user.handle }}
              </p>
              <p
                v-if="user.bio"
                class="public-profile-bio mt-2 line-clamp-3 max-w-[62ch] whitespace-pre-line text-pretty text-sm leading-6 sm:text-base"
              >
                {{ user.bio }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div
        class="flex min-h-16 flex-col gap-4 px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-7"
      >
        <div class="min-w-0">
          <p class="text-xs font-medium text-muted">永久用户号</p>
          <NuxtLink
            :to="stableProfilePath"
            class="mt-0.5 inline-flex min-h-7 max-w-full items-center gap-2 rounded-sm font-mono text-sm font-semibold text-highlighted outline-none transition-colors hover:text-primary focus-visible:ring-2 focus-visible:ring-primary"
            :aria-label="'打开 ' + user.displayName + ' 的永久资料地址'"
          >
            <UIcon name="i-tabler-link" class="size-4 shrink-0 text-primary" />
            <span class="min-w-0 break-all">{{ user.userKey }}</span>
          </NuxtLink>
        </div>

        <nav
          v-if="links.length"
          class="flex min-w-0 flex-wrap gap-1.5 sm:justify-end"
          aria-label="公开链接"
        >
          <UButton
            v-for="link in links"
            :key="link.label + ':' + link.url"
            :to="link.url"
            external
            target="_blank"
            rel="noopener noreferrer"
            color="neutral"
            variant="soft"
            size="sm"
            :icon="link.platform.icon"
            trailing-icon="i-tabler-arrow-up-right"
            :label="link.label"
            class="min-h-9"
          />
        </nav>
      </div>
    </section>
  </article>
</template>

<style scoped>
.public-profile-cover-fallback {
  background: linear-gradient(
    135deg,
    color-mix(in oklab, var(--ui-primary) 34%, transparent),
    color-mix(in oklab, var(--ui-primary) 12%, var(--ui-bg-elevated)) 52%,
    var(--ui-bg-elevated)
  );
}

.public-profile-shade {
  background: linear-gradient(
    to top,
    rgb(0 0 0 / 0.84) 0%,
    rgb(0 0 0 / 0.34) 46%,
    rgb(0 0 0 / 0.06) 100%
  );
}

.public-profile-handle {
  color: rgb(255 255 255 / 0.76);
}

.public-profile-bio {
  color: rgb(255 255 255 / 0.88);
}
</style>
