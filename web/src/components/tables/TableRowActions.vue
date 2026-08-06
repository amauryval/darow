<script setup lang="ts">
import { Copy, Trash2 } from '@lucide/vue'
import BaseButton from '../ui/BaseButton.vue'
import ConfirmButton from '../ui/ConfirmButton.vue'

// What a single click does to one row, on the spot: duplicate it, delete it.
// Reading its geometry is not here — that is a column of its own, headed by
// the geometry column's name (`TableGeometryCell`). Retyping is not one of them — the mode makes every
// cell writable and the session's own *Enregistrer* writes them all, so there
// is no per-row edit/save/cancel trio to fill this column with (§8.8).
//
// Icons alone, at the **start** of the row, next to its number: they are the
// same three actions on every line, so naming each one on each line said
// nothing and pushed them to the far right of a grid that scrolls sideways —
// where they were reached only after scrolling past every column. Each keeps
// its accessible name and its tooltip (§8.7), and deleting keeps its
// confirmation (§8.3).
//
// There is no geometry *edit* action here on purpose: geometries are edited
// in one place, the map, through one button (§8.8) — a second door onto the
// same mode is a second thing to explain.
withDefaults(
    defineProps<{
        // A row the session is about to create: it exists nowhere yet, so the
        // only thing that applies to it is dropping it.
        draft?: boolean
        editable?: boolean
        isDeletingPkey?: string
        isDuplicating?: boolean
        deletingPkey?: string
    }>(),
    { draft: false, editable: false, isDuplicating: false }
)

defineEmits<{
    duplicate: []
    delete: []
}>()
</script>

<template>
    <div class="flex items-center gap-0.5">
        <ConfirmButton
            v-if="draft"
            label="Retirer cette ligne"
            variant="ghost"
            :icon="Trash2"
            icon-only
            message="Retirer cette ligne, avec ce qui y a été saisi ?"
            @confirm="$emit('delete')"
        />
        <template v-if="editable">
            <!-- Duplicating is not confirmed: it adds, and what it added is
           deleted by the button next to it (§8.3). -->
            <BaseButton
                size="xs"
                variant="ghost"
                :icon="Copy"
                icon-only
                :busy="isDuplicating"
                :disabled="isDuplicating"
                title="Dupliquer cette ligne"
                @click="$emit('duplicate')"
            >
                Dupliquer
            </BaseButton>
            <ConfirmButton
                label="Supprimer cette ligne"
                variant="ghost"
                :icon="Trash2"
                icon-only
                message="Supprimer définitivement cette ligne ?"
                :busy="isDeletingPkey === deletingPkey"
                @confirm="$emit('delete')"
            />
        </template>
    </div>
</template>
