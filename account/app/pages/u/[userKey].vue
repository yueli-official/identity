<script setup lang="ts">
import type { PublicUserResponse } from "~/types/public-user";

definePageMeta({
  key: (route) => route.fullPath,
});

const route = useRoute();
const { call } = useApi();
const userKey = String(route.params.userKey || "");
const endpoint = `/api/v1/users/${encodeURIComponent(userKey)}`;
const { data, error } = await useAsyncData(
  `public-user-key:${userKey}`,
  () => call<PublicUserResponse>(endpoint),
);

if (error.value) {
  const status = Number(error.value.statusCode || 500);
  if (status === 400 || status === 404) {
    throw createError({
      statusCode: 404,
      statusMessage: "Not Found",
      message: "没有找到这位用户。",
    });
  }
  throw error.value;
}

const user = data.value?.user;
if (!user) {
  throw createError({
    statusCode: 404,
    statusMessage: "Not Found",
    message: "没有找到这位用户。",
  });
}

const canonicalPath = user.handle
  ? `/@${encodeURIComponent(user.handle)}`
  : `/u/${encodeURIComponent(user.userKey)}`;

useSeoMeta({
  title: `${user.displayName}${user.handle ? ` (@${user.handle})` : ""} · 账户中心`,
  description: user.bio || `查看 ${user.displayName} 的公开资料。`,
});
useHead({ link: [{ rel: "canonical", href: canonicalPath }] });
</script>

<template>
  <PublicUserProfile :user="user" />
</template>
