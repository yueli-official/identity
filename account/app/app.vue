<script setup lang="ts">
import { FeedbackToastRegion } from '@yueli/ui/feedback/pattern'

useHead({ htmlAttrs: { lang: 'zh-CN' } })
</script>

<template>
  <UApp :toaster="null">
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <FeedbackToastRegion />
  </UApp>
</template>
