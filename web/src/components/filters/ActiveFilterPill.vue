<script setup lang="ts">
import { Pencil, X } from '@lucide/vue'
import type { SavedFilter } from '../../client/types'
import Badge from '../ui/Badge.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// Only the applied filter, not the whole catalogue: a single filter applies at
// a time, and SavedFilterMenu already lists them all — a pill row repeating it
// showed every filter twice. The ✕ un-applies, it never deletes: destroying a
// stored filter belongs to the menu, a click away from the toolbar.
defineProps<{ filter: SavedFilter }>()
const emit = defineEmits<{ clear: []; edit: [] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()
</script>

<template>
    <Badge tone="primary">
        <span class="max-w-64 truncate py-0.5" :title="filter.name">{{ filter.name }}</span>
        <button
            type="button"
            class="rounded-full p-0.5 hover:bg-black/10 disabled:opacity-40"
            aria-label="Modifier le filtre appliqué"
            :title="writeDisabled ? writeGuardReason : 'Modifier le filtre appliqué'"
            :disabled="writeDisabled"
            @click="emit('edit')"
        >
            <Pencil :size="12" aria-hidden="true" />
        </button>
        <button
            type="button"
            class="rounded-full p-0.5 hover:bg-black/10"
            aria-label="Retirer le filtre"
            title="Retirer le filtre (il reste enregistré)"
            @click="emit('clear')"
        >
            <X :size="12" aria-hidden="true" />
        </button>
    </Badge>
</template>
