<script setup lang="ts">
import type { Component } from "vue";

const props = withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "danger" | "ghost";
    size?: "xs" | "sm" | "md";
    square?: boolean;
    icon?: Component;
    // Shows the icon alone, the label becoming visually hidden but still
    // read by assistive tech — the caller must then pass ariaLabel (§8.7).
    iconOnly?: boolean;
    busy?: boolean;
    disabled?: boolean;
    ariaLabel?: string;
    title?: string;
  }>(),
  { variant: "secondary", size: "md" },
);

const variants = {
  primary: "btn-primary",
  secondary: "btn-outline",
  // Filled, never outlined: the theme draws a button's border and shadow on
  // the solid variants only, so an outlined red button lost both and read as
  // a foreign control next to the buttons it stands with (§7).
  danger: "btn-error",
  ghost: "btn-ghost",
};

const sizes = {
  xs: "btn-xs",
  sm: "btn-sm",
  md: "btn-md",
};

const iconSizes = {
  xs: 12,
  sm: 14,
  md: 16,
};
</script>

<template>
  <button
    :disabled="busy || disabled"
    :aria-label="ariaLabel"
    :title="title"
    :class="[
      'btn',
      variants[variant],
      sizes[size],
      { 'btn-square': square || (icon && iconOnly) },
    ]"
  >
    <span v-if="busy" class="loading loading-spinner loading-xs"></span>
    <component :is="icon" v-else-if="icon" :size="iconSizes[props.size]" aria-hidden="true" />
    <span v-if="!(icon && iconOnly)"><slot /></span>
  </button>
</template>
