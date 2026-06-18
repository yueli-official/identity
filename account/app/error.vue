<script setup lang="ts">
defineProps<{ error: { statusCode: number; message: string } }>()
</script>
<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm text-center">
      <h1 class="font-display text-2xl font-semibold text-highlighted">{{ error.statusCode }}</h1>
      <p class="mt-2 text-sm text-muted">{{ error.message }}</p>
      <UButton variant="link" label="回首页" class="mt-4" @click="clearError({ redirect: '/' })" />
    </UCard>
  </div>
</template>
