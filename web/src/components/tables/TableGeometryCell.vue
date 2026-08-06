<script setup lang="ts">
import { Shapes } from '@lucide/vue'
import BaseButton from '../ui/BaseButton.vue'

// The geometry column, as a column. Its value is never in the grid — a WKT
// literal is unreadable at cell width and no row listing serves it (LIM-F3) —
// but the column exists all the same, under its own name and in its own
// place, and its cell is what opens the geometry in both encodings. Reading a
// geometry is a read: it is offered whether or not the grid is armed.
defineEmits<{ open: [] }>()
</script>

<template>
    <BaseButton
        size="xs"
        variant="ghost"
        :icon="Shapes"
        icon-only
        title="Voir la géométrie (WKT, GeoJSON)"
        @click="$emit('open')"
    >
        Voir la géométrie
    </BaseButton>
</template>
