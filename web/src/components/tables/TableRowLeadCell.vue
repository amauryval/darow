<script setup lang="ts">
import TableRowActions from './TableRowActions.vue'

// The row's own cell: what names it and what acts on it, together in the
// grid's first column. They belong to the same register — neither is a value
// of the table — and a column of their own for two icons pushed every
// attribute one step further right for nothing.
//
// The rank is the row's position in the grid **as displayed**, selection
// pinned at the top included: it is what a user points at when they say "the
// third one", and it is not the primary key — a pkey is an identifier, not a
// position, and the two disagree as soon as the grid is sorted. A row the
// session has yet to create holds no rank at all.
defineProps<{
    rank: number | null
    draft?: boolean
    editable?: boolean
    isDeletingPkey?: string
    isDuplicating?: boolean
    deletingPkey?: string
}>()

defineEmits<{ duplicate: []; delete: [] }>()
</script>

<template>
    <!-- Actions first, rank last: the rank is read against the row it numbers,
       so it stays next to the values, and the controls sit at the edge of the
       grid where the pointer reaches them without crossing anything. -->
    <div class="flex items-center justify-end gap-1">
        <TableRowActions
            v-if="draft || editable"
            :draft="draft"
            :editable="editable"
            :is-deleting-pkey="isDeletingPkey"
            :is-duplicating="isDuplicating"
            :deleting-pkey="deletingPkey"
            @duplicate="$emit('duplicate')"
            @delete="$emit('delete')"
        />
        <span class="tabular-nums text-base-content/40">{{ rank ?? '' }}</span>
    </div>
</template>
