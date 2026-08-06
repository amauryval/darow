<script setup lang="ts">
import { formatSize } from '../../utils/sourceRows'

// A multi-layer file shows its layer count next to its own size; a
// single-layer file or a plain layer shows its size alone — a layer row's own
// size is not always known, in which case nothing renders.
defineProps<{ sizeBytes?: number; layerCount?: number }>()
</script>

<template>
    <span
        v-if="layerCount !== undefined && sizeBytes !== undefined"
        class="text-xs text-base-content/50"
    >
        {{ layerCount }} couches · {{ formatSize(sizeBytes) }}
    </span>
    <span
        v-else-if="layerCount === undefined && sizeBytes !== undefined"
        class="text-base-content/60"
    >
        {{ formatSize(sizeBytes) }}
    </span>
</template>
