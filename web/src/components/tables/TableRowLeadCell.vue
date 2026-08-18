<script setup lang="ts">
import { Trash2 } from '@lucide/vue'
import ConfirmButton from '../ui/ConfirmButton.vue'

// The row's own cell: what names it, and — for a row the session has yet to
// create — the one control that applies to it. Duplicating and deleting a
// written row are not here: they act on the selection, from the bar above the
// grid, so that one row and twenty are the same gesture (§8.6).
//
// The rank is the row's position in the loaded page as displayed: it is what a
// user points at when they say "the third one", and it is not the primary key —
// a pkey is an identifier, not a position, and the two disagree as soon as the
// grid is sorted. A pinned row holds no rank at all: its position is borrowed.
defineProps<{
    rank: number | null
    // A row the session is about to create: it exists nowhere yet, so the only
    // thing that applies to it is dropping it.
    draft?: boolean
}>()

defineEmits<{ delete: [] }>()
</script>

<template>
    <!-- Action first, rank last: the rank is read against the row it numbers,
       so it stays next to the values, and the control sits at the edge of the
       grid where the pointer reaches it without crossing anything. -->
    <div class="flex items-center justify-end gap-1">
        <ConfirmButton
            v-if="draft"
            label="Retirer cette ligne"
            variant="ghost"
            :icon="Trash2"
            icon-only
            message="Retirer cette ligne, avec ce qui y a été saisi ?"
            @confirm="$emit('delete')"
        />
        <span class="tabular-nums text-base-content/40">{{ rank ?? '' }}</span>
    </div>
</template>
