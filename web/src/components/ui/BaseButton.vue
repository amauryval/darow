<script setup lang="ts">
import type { Component } from "vue";
import Spinner from "./Spinner.vue";

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
    // Set on every button of a ButtonGroup: daisyUI welds a row of controls
    // by the modifier each item carries, not by the wrapper alone.
    joined?: boolean;
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
      { 'btn-square': square || (icon && iconOnly), 'join-item': joined },
    ]"
  >
    <Spinner v-if="busy" />
    <component :is="icon" v-else-if="icon" :size="iconSizes[props.size]" aria-hidden="true" />
    <span v-if="!(icon && iconOnly)"><slot /></span>
  </button>
</template>
