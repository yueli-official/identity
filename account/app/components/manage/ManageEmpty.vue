<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    icon?: string;
    text?: string;
    title?: string;
    description?: string;
  }>(),
  {
    icon: "i-tabler-inbox",
    text: "",
    title: "",
    description: "",
  },
);

const heading = computed(() => props.title || props.text);
</script>

<template>
  <div class="rounded-2xl border border-dashed border-default py-16 text-center">
    <div class="mx-auto grid size-14 place-items-center rounded-2xl bg-primary/10 text-primary">
      <UIcon :name="icon" class="size-7" />
    </div>
    <p v-if="$slots.default || heading" class="mt-4 text-sm font-medium text-highlighted">
      <slot>{{ heading }}</slot>
    </p>
    <p v-if="description" class="mx-auto mt-1 max-w-xl text-sm text-muted">
      {{ description }}
    </p>
  </div>
</template>
