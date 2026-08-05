<script setup lang="ts">
import { ref, useId } from "vue";
import type { Component } from "vue";
import { onClickOutside } from "@vueuse/core";
import BaseButton from "./BaseButton.vue";

// daisyUI `dropdown` + `menu`: a trigger button and the panel it opens under
// itself. The open state is held here rather than left to daisyUI's CSS-focus
// or `<details>` variants: both only close when focus leaves, so acting from
// the panel left it hanging over the toolbar, and opening a modal put it
// behind a still-open menu. The content is the caller's — this owns the
// chrome, the anchoring and the closing.
withDefaults(
  defineProps<{
    label: string;
    icon?: Component;
    iconOnly?: boolean;
    ariaLabel?: string;
    title?: string;
    variant?: "primary" | "secondary" | "danger" | "ghost";
    size?: "xs" | "sm" | "md";
    // Which edge of the trigger the panel hangs from: only the caller knows
    // whether it stands at the left of a bar (where a right-anchored panel
    // would run off-screen) or at its right (where a left-anchored one would).
    align?: "left" | "right";
    // Width and max height of the panel — its content is the caller's, so its
    // size is too.
    panelClass?: string;
  }>(),
  { variant: "secondary", size: "sm", align: "left" },
);

const root = ref<HTMLElement | null>(null);
const open = ref(false);
const panelId = useId();

onClickOutside(root, () => (open.value = false));

function close() {
  open.value = false;
}
</script>

<template>
  <div
    ref="root"
    class="dropdown dropdown-bottom"
    :class="[align === 'right' ? 'dropdown-end' : 'dropdown-start', { 'dropdown-open': open }]"
    @keydown.esc="close"
  >
    <BaseButton
      :variant="variant"
      :size="size"
      :icon="icon"
      :icon-only="iconOnly"
      :aria-label="ariaLabel"
      :title="title"
      :aria-expanded="open"
      :aria-controls="panelId"
      aria-haspopup="true"
      @click="open = !open"
    >
      <slot name="label">{{ label }}</slot>
    </BaseButton>

    <ul
      v-if="open"
      :id="panelId"
      class="dropdown-content menu menu-sm z-30 mt-1 min-w-52 flex-nowrap overflow-y-auto rounded-box border border-base-300 bg-base-100 p-1 shadow-sm"
      :class="panelClass"
    >
      <slot :close="close" />
    </ul>
  </div>
</template>
