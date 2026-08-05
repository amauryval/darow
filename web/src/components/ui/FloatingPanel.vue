<script setup lang="ts">
// A panel floating over the content behind it — the map's own chrome: the
// render toggle in a corner, the edit bar along the bottom. It is positioned
// by its parent's box, so the caller only says where, never how.
withDefaults(defineProps<{ position?: "top-left" | "bottom" }>(), { position: "top-left" });

const positions = {
  "top-left": "top-2 left-2",
  bottom: "inset-x-2 bottom-2",
};
</script>

<template>
  <div class="card absolute z-10 bg-base-100 px-3 py-2 shadow" :class="positions[position]">
    <slot />
  </div>
</template>
