<script setup lang="ts">
withDefaults(defineProps<{ tone?: "primary" | "success" | "neutral" | "warning" | "error" }>(), { tone: "neutral" });

const classes: Record<string, string> = {
  primary: "badge-primary",
  success: "badge-success",
  neutral: "badge-neutral",
  // Amber, the colour a pending write is shown in everywhere — the edit bar's
  // count of touched geometries reads the same as the rest of the app.
  warning: "badge-warning",
  error: "badge-error",
};
</script>

<template>
  <span class="badge badge-soft badge-sm font-medium" :class="classes[tone]">
    <slot />
  </span>
</template>
