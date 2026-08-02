<script setup lang="ts">
withDefaults(defineProps<{ tone?: "primary" | "success" | "neutral" | "error" }>(), { tone: "neutral" });

const classes: Record<string, string> = {
  primary: "badge-primary",
  success: "badge-success",
  neutral: "badge-neutral",
  error: "badge-error",
};
</script>

<template>
  <span class="badge badge-soft badge-sm font-medium" :class="classes[tone]">
    <slot />
  </span>
</template>
