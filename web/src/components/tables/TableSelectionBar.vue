<script setup lang="ts">
import { computed } from 'vue'
import { ArrowUpToLine, Copy, ListFilter, Maximize, Trash2, X } from '@lucide/vue'
import type { SelectionView } from '../../composables/useSelectionView'
import BaseButton from '../ui/BaseButton.vue'
import ButtonGroup from '../ui/ButtonGroup.vue'
import ConfirmButton from '../ui/ConfirmButton.vue'
import Divider from '../ui/Divider.vue'
import Typography from '../ui/Typography.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// The bar a selection raises: how many rows are held, and everything that acts
// on them. Selecting is one gesture — a click on a row, a click on its feature
// — so what it enables stands in one place rather than split between the
// table's identity line and each row's own cell (§8.6).
//
// It **floats** over the panels rather than taking a row of its own: a bar that
// appears and disappears in the layout resized the grid and the map under the
// pointer at the exact moment the user was clicking rows, and the next click
// landed somewhere else. Positioning is the caller's (`TableDataPanel`), which
// is what knows the area it must not push around.
//
// It is not an edit-session bar: duplicating and deleting are single confirmed
// writes with their own feedback, not drafts a *Enregistrer* commits (LIM-F10),
// so they do not wait for the mode that makes cells writable.
const props = defineProps<{
    count: number
    // Framing the map on the selection only means something where there is a
    // map to frame.
    isGeo: boolean
    zooming: boolean
    duplicating: boolean
    deleting: boolean
    // How the grid currently arranges the selection. The two controls are
    // toggles onto it, not a third and fourth state of their own.
    view: SelectionView
}>()
defineEmits<{
    view: [mode: 'pinned' | 'only']
    clear: []
    zoom: []
    duplicate: []
    delete: []
}>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

const plural = computed(() => (props.count > 1 ? 's' : ''))
const busy = computed(() => props.duplicating || props.deleting)
</script>

<template>
    <div
        class="pointer-events-auto flex flex-wrap items-center gap-x-3 gap-y-2 rounded-full border border-base-300 bg-base-100 px-4 py-2 shadow-lg"
        role="group"
        aria-label="Actions sur la sélection"
    >
        <!-- Amber, the colour a selected row is drawn in on both halves of the
           screen: the bar and what it holds are read as the same state. -->
        <span class="size-2.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />
        <Typography variant="body-sm">
            {{ count.toLocaleString('fr-FR') }} ligne{{ plural }} sélectionnée{{ plural }}
        </Typography>

        <BaseButton
            v-if="isGeo"
            size="xs"
            variant="ghost"
            :icon="Maximize"
            icon-only
            :busy="zooming"
            title="Cadrer sur la sélection"
            @click="$emit('zoom')"
        >
            Cadrer sur la sélection
        </BaseButton>

        <!-- A selected row stays where the sort put it; these two ask for it to
           be moved, and they are also the only way a row selected on the map,
           outside the loaded pages, is reached in the grid (§8.6). -->
        <ButtonGroup>
            <BaseButton
                size="xs"
                joined
                :variant="view === 'pinned' ? 'primary' : 'ghost'"
                :icon="ArrowUpToLine"
                icon-only
                :aria-pressed="view === 'pinned'"
                title="Remonter la sélection en haut de la liste"
                @click="$emit('view', 'pinned')"
            >
                Remonter la sélection en haut de la liste
            </BaseButton>
            <BaseButton
                size="xs"
                joined
                :variant="view === 'only' ? 'primary' : 'ghost'"
                :icon="ListFilter"
                icon-only
                :aria-pressed="view === 'only'"
                title="N'afficher que la sélection"
                @click="$emit('view', 'only')"
            >
                N'afficher que la sélection
            </BaseButton>
        </ButtonGroup>

        <Divider />

        <!-- Duplicating is not confirmed: it adds, and what it added is deleted
           by the button next to it (§8.3). -->
        <BaseButton
            size="xs"
            :icon="Copy"
            :busy="duplicating"
            :disabled="writeDisabled || busy"
            :title="writeDisabled ? writeGuardReason : undefined"
            @click="$emit('duplicate')"
        >
            Dupliquer
        </BaseButton>
        <ConfirmButton
            :label="`Supprimer ${count} ligne${plural}`"
            confirm-label="Supprimer"
            :message="`Supprimer définitivement ${count} ligne${plural} ? Cette action est immédiate et ne fait pas partie de la session d'édition.`"
            :icon="Trash2"
            :busy="deleting"
            :disabled="writeDisabled || busy"
            :title="writeDisabled ? writeGuardReason : undefined"
            @confirm="$emit('delete')"
        />

        <Divider />

        <BaseButton
            size="xs"
            variant="ghost"
            :icon="X"
            icon-only
            title="Vider la sélection"
            @click="$emit('clear')"
        >
            Vider la sélection
        </BaseButton>
    </div>
</template>
