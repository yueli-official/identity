<script setup lang="ts">
withDefaults(defineProps<{ rows?: number }>(), { rows: 6 });
</script>

<template>
  <div data-manage-skeleton="list" class="divide-y divide-default overflow-hidden rounded-2xl border border-default">
    <div v-for="index in rows" :key="index" class="flex items-center gap-3 px-4 py-3">
      <USkeleton class="size-9 shrink-0 rounded-full" />
      <div class="min-w-0 flex-1 space-y-2">
        <USkeleton class="h-3.5 w-40 max-w-[55%]" />
        <USkeleton class="h-3 w-24 max-w-[30%]" />
      </div>
      <USkeleton class="h-5 w-14 shrink-0 rounded-full" />
    </div>
  </div>
</template>
