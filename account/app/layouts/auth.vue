<template>
  <!-- Minimal shell for pre-login pages: page bg + a color-mode toggle in the
       corner. Pages center their own card within this. -->
  <div class="relative min-h-dvh bg-default text-default">
    <div class="absolute right-4 top-4 z-10">
      <UColorModeButton />
    </div>
    <slot />
  </div>
</template>
