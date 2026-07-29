<script setup lang="ts">
withDefaults(defineProps<{ tone?: "indigo" | "emerald" | "slate" | "red" }>(), { tone: "slate" });

const classes: Record<string, string> = {
  indigo: "badge-primary",
  emerald: "badge-success",
  slate: "badge-neutral",
  red: "badge-error",
};
</script>

<template>
  <span class="badge badge-soft badge-sm font-medium" :class="classes[tone]">
    <slot />
  </span>
</template>
