<script setup lang="ts">
import type { Component } from "vue";

// One `<li>` of a daisyUI `menu` — the shape `MenuDropdown`'s panel expects.
// A menu entry is a full-width target, never a `BaseButton`: a stack of
// bordered buttons inside a panel reads as a form, not as a menu. The
// `actions` slot is what a row carries at its right (edit, delete): they sit
// beside the entry, not inside it, so clicking one never triggers the entry.
// The `<li>` is daisyUI's own layout wrapper, so everything the caller sets —
// the click handler, a title, an aria-pressed — belongs on the button inside
// it, not on the list item.
defineOptions({ inheritAttrs: false });

withDefaults(
  defineProps<{
    icon?: Component;
    // The entry whose state is currently applied — daisyUI's own `menu-active`.
    active?: boolean;
    danger?: boolean;
    disabled?: boolean;
  }>(),
  { active: false, danger: false, disabled: false },
);
</script>

<template>
  <li class="flex-row flex-nowrap items-center gap-0.5" :class="{ 'menu-disabled': disabled }">
    <button
      v-bind="$attrs"
      type="button"
      :disabled="disabled"
      class="min-w-0 flex-1"
      :class="{ 'menu-active': active, 'text-error': danger }"
    >
      <component :is="icon" v-if="icon" :size="14" aria-hidden="true" class="shrink-0" />
      <span class="min-w-0 flex-1 truncate text-left"><slot /></span>
    </button>
    <slot name="actions" />
  </li>
</template>
