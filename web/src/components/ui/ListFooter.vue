<script setup lang="ts">
import type { Ref } from 'vue'

// The infinite-scroll sentinel, its "Chargement…" indicator and the count of
// what is on screen against what the server holds, worded and styled once
// (§0.8) — usePaginatedList's own observer target and its `total`, dropped
// at the foot of whatever list it paginates. The count is what tells a
// complete list from its first screen (§8.6).
defineProps<{
    sentinel: Ref<HTMLElement | null>
    loading?: boolean
    loaded: number
    total: number
}>()
</script>

<template>
    <div :ref="sentinel" class="h-px"></div>
    <p v-if="loading" class="px-4 py-3 text-center text-xs text-base-content/40">Chargement…</p>
    <p v-else-if="total > 0" class="px-4 py-3 text-center text-xs text-base-content/40">
        {{ loaded.toLocaleString('fr-FR') }} sur {{ total.toLocaleString('fr-FR') }}
    </p>
</template>
