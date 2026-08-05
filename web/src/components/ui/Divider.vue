<script setup lang="ts">
// daisyUI `divider`, tightened: the line that says the controls on either
// side belong to two different registers. daisyUI names the class after the
// **layout** it divides, not after the line it draws — `divider-horizontal`
// is the vertical rule between two groups of a toolbar row — so the prop is
// named after the line and mapped here, once.
withDefaults(defineProps<{ orientation?: "vertical" | "horizontal" }>(), { orientation: "vertical" });
</script>

<template>
  <div class="divider" :class="orientation === 'vertical' ? 'divider-horizontal mx-0 h-6 self-center' : 'my-0'" />
</template>
