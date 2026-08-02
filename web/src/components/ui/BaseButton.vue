<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "danger";
    size?: "sm" | "md";
    busy?: boolean;
    disabled?: boolean;
  }>(),
  { variant: "secondary", size: "md" },
);

const variants = {
  primary: "bg-indigo-600 text-white hover:bg-indigo-500 disabled:opacity-50",
  secondary: "border border-slate-300 bg-white text-slate-700 hover:bg-slate-50",
  danger: "bg-red-600 text-white hover:bg-red-500 disabled:opacity-50",
};

const sizes = {
  sm: "px-2.5 py-1 text-xs font-medium",
  md: "px-3 py-1.5 text-sm font-medium",
};
</script>

<template>
  <button
    :disabled="busy || disabled"
    :class="[
      'rounded-md transition-colors disabled:cursor-not-allowed',
      variants[variant],
      sizes[size],
    ]"
  >
    <slot />
  </button>
</template>
