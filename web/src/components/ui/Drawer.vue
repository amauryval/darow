<script setup lang="ts">
import { useId } from "vue";

// daisyUI `drawer` as a closeable side panel. `.drawer` is laid out as the
// region it wraps (display: grid), not shrink-wrapped around a trigger, so
// the caller puts the whole content region in the default slot — its own
// trigger included, wired to this same v-model — exactly as App.vue's nav
// drawer wraps the page. The side panel's content goes in the `panel` slot.
const open = defineModel<boolean>({ required: true });

withDefaults(defineProps<{ end?: boolean; width?: string }>(), { end: true, width: "w-80" });

const toggleId = useId();
</script>

<template>
  <div class="drawer" :class="{ 'drawer-end': end }">
    <input :id="toggleId" v-model="open" type="checkbox" class="drawer-toggle" />

    <div class="drawer-content flex min-h-0 flex-col">
      <slot />
    </div>

    <div class="drawer-side z-30">
      <label :for="toggleId" class="drawer-overlay" aria-label="Fermer le panneau"></label>
      <div class="min-h-full bg-base-200 p-4" :class="width">
        <slot name="panel" />
      </div>
    </div>
  </div>
</template>
