<script setup lang="ts">
// The busy indicator, wherever something is in flight and the surrounding
// content stays on screen — a skeleton is for a first load, this is for a
// refresh happening next to what it refreshes (§8.1).
withDefaults(defineProps<{ size?: "xs" | "sm" | "md"; label?: string }>(), { size: "xs" });

const sizes = {
  xs: "loading-xs",
  sm: "loading-sm",
  md: "loading-md",
};
</script>

<template>
  <span class="loading loading-spinner" :class="sizes[size]" role="status" :aria-label="label ?? 'Chargement en cours'" />
</template>
