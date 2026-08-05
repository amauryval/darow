<script setup lang="ts">
import { EDIT_LEGEND } from "../../utils/drawStyles";
import FloatingPanel from "../ui/FloatingPanel.vue";
import Typography from "../ui/Typography.vue";

// What the session's colours mean, stated on the map that carries them rather
// than in the bar above it: a legend read next to the shapes it names costs
// nothing to find, and the bar stays one row of controls (§8.7 — none of it
// rests on colour alone).
</script>

<template>
  <FloatingPanel position="bottom-left">
    <ul class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <li v-for="entry in EDIT_LEGEND" :key="entry.label" class="flex items-center gap-1">
        <span class="size-2.5 shrink-0 rounded-full" :style="{ backgroundColor: entry.tone }" aria-hidden="true" />
        <Typography variant="caption">{{ entry.label }}</Typography>
      </li>
      <!-- The one gesture of the session that is not on a button, and the
           only place it can be read: outside a session a plain click selects,
           under one it belongs to the editor. -->
      <li class="flex items-center gap-1 border-l border-base-300 pl-3">
        <Typography variant="caption"><kbd class="kbd kbd-xs">Maj</kbd> + clic : sélectionner</Typography>
      </li>
    </ul>
  </FloatingPanel>
</template>
