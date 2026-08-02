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
  primary: "btn-primary",
  secondary: "btn-secondary",
  danger: "btn-danger",
};

const sizes = {
  sm: "btn-sm",
  md: "btn-md",
};
</script>

<template>
  <button
    :disabled="busy || disabled"
    :class="[
      'btn',
      variants[variant],
      sizes[size],
    ]"
  >
    <slot />
  </button>
</template>
