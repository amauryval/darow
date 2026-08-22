<script setup lang="ts">
import { Columns3, LayoutPanelLeft, Map as MapIcon, PencilLine, Table2 } from '@lucide/vue'
import type { ColumnSchema } from '../../client/types'
import type { DraftNode } from '../../utils/filterTree'
import TableFilterBar from '../filters/TableFilterBar.vue'
import BaseButton from '../ui/BaseButton.vue'
import ButtonGroup from '../ui/ButtonGroup.vue'
import Divider from '../ui/Divider.vue'
import Typography from '../ui/Typography.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// The workspace's bar: everything that acts on what is *displayed*, in the
// order it is read — which rows (les données), what is being written
// (l'édition), how it is laid out (l'affichage). What acts on the table as an
// object lives one row above, in `TableHeader`: a bar that mixes "exporter la
// table" with "maximiser la carte" leaves the user reading five registers at
// once (§8.9).
defineProps<{
    tableId: string
    columns: ColumnSchema[]
    isGeo: boolean
    editing: boolean
    focusPanel: 'split' | 'table' | 'map'
    activeFilterId: string | null
}>()
const emit = defineEmits<{
    'update:nodes': [nodes: DraftNode[]]
    'update:focusPanel': [panel: 'split' | 'table' | 'map']
    'update:activeFilterId': [id: string | null]
    toggleEditing: []
    openColumns: []
}>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard('data')
</script>

<template>
    <div class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2">
        <TableFilterBar
            :table-id="tableId"
            :columns="columns"
            :active-filter-id="activeFilterId"
            @update:nodes="emit('update:nodes', $event)"
            @update:active-filter-id="emit('update:activeFilterId', $event)"
        />

        <!-- Editing is one mode of the table, armed here and detailed by the bar
         it raises above both panels: the rows and the geometries are two
         halves of the same object, not two modes to choose between. -->
        <div class="ml-auto flex items-center gap-2">
            <BaseButton
                size="sm"
                :variant="editing ? 'primary' : 'secondary'"
                :icon="PencilLine"
                :aria-pressed="editing"
                :disabled="!editing && writeDisabled"
                :title="
                    !editing && writeDisabled
                        ? writeGuardReason
                        : editing
                          ? 'Quitter le mode édition'
                          : 'Modifier les lignes et les géométries de cette table'
                "
                @click="emit('toggleEditing')"
            >
                Éditer
            </BaseButton>
        </div>

        <Divider />

        <div class="flex items-center gap-2">
            <Typography variant="eyebrow">Affichage</Typography>
            <BaseButton
                size="sm"
                :icon="Columns3"
                title="Choisir les colonnes affichées"
                @click="emit('openColumns')"
            >
                Colonnes
            </BaseButton>

            <!-- Table and map default to half the width each; either can be
           maximized when the data being read needs the room (§8.10). -->
            <ButtonGroup v-if="isGeo">
                <BaseButton
                    joined
                    size="sm"
                    :variant="focusPanel === 'table' ? 'primary' : 'secondary'"
                    :icon="Table2"
                    icon-only
                    aria-label="Afficher le tableau seul"
                    :aria-pressed="focusPanel === 'table'"
                    title="Afficher le tableau seul"
                    @click="emit('update:focusPanel', focusPanel === 'table' ? 'split' : 'table')"
                />
                <BaseButton
                    joined
                    size="sm"
                    :variant="focusPanel === 'split' ? 'primary' : 'secondary'"
                    :icon="LayoutPanelLeft"
                    icon-only
                    aria-label="Partager la vue entre tableau et carte"
                    :aria-pressed="focusPanel === 'split'"
                    title="Partager la vue entre tableau et carte"
                    @click="emit('update:focusPanel', 'split')"
                />
                <BaseButton
                    joined
                    size="sm"
                    :variant="focusPanel === 'map' ? 'primary' : 'secondary'"
                    :icon="MapIcon"
                    icon-only
                    aria-label="Afficher la carte seule"
                    :aria-pressed="focusPanel === 'map'"
                    title="Afficher la carte seule"
                    @click="emit('update:focusPanel', focusPanel === 'map' ? 'split' : 'map')"
                />
            </ButtonGroup>
        </div>
    </div>
</template>
