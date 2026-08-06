<script setup lang="ts">
import { Plus } from '@lucide/vue'
import BaseButton from '../ui/BaseButton.vue'
import EditSessionActions from './EditSessionActions.vue'
import Typography from '../ui/Typography.vue'

// The bar an edit session raises above **both** panels: editing is one mode
// of the table, and what it offers is what the table can carry. On a geo
// table that is the map's own tools, creating included — a row is created by
// drawing its shape, so *Créer* is where the shapes are. A table with nothing
// to draw keeps the same group with the one button that still means
// something: a `+` that raises an empty row in the grid (§8.3).
//
// The map's own tools are rendered into `toolsId` by `MapView` (a Teleport):
// they need the OpenLayers session that lives inside the map component, which
// must not be pulled into the initial bundle by this bar (§8.10). The
// session's *Enregistrer* / *Annuler* travel with them, so that they stand
// next to the geometry counts only the map knows; a table without geometry
// has no such Teleport, and the pair is rendered here.
defineProps<{
    isGeo: boolean
    toolsId: string
    rowCount: number
    draftCount: number
    saving?: boolean
}>()
defineEmits<{ addRow: []; save: []; cancel: [] }>()
</script>

<template>
    <div
        class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2"
    >
        <div v-if="isGeo" :id="toolsId" class="flex min-w-0 flex-1 items-center" />

        <template v-else>
            <!-- Secondary, not primary: the bar's one filled button is
           *Enregistrer*, which is what ends the session. -->
            <div class="flex items-center gap-1.5">
                <Typography variant="eyebrow">Créer</Typography>
                <BaseButton
                    size="sm"
                    :icon="Plus"
                    title="Ajouter une ligne vide, à remplir dans la grille"
                    @click="$emit('addRow')"
                    iconOnly
                />
            </div>
            <EditSessionActions
                class="ml-auto"
                :changed-count="0"
                :created-count="draftCount"
                :row-count="rowCount"
                :saving="saving"
                @save="$emit('save')"
                @cancel="$emit('cancel')"
            />
        </template>
    </div>
</template>
