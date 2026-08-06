<script setup lang="ts">
import type { Ref } from "vue";

// The infinite-scroll sentinel plus its "Chargement…" indicator, worded and
// styled once (§0.8) — usePaginatedList's own observer target, dropped at
// the foot of whatever list it paginates.
defineProps<{ sentinel: Ref<HTMLElement | null>; loading?: boolean }>();
</script>

<template>
  <div :ref="sentinel" class="h-px"></div>
  <p v-if="loading" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
</template>
