<script setup lang="ts">
import { toRef } from 'vue'
import type { ExportFormat, Metadata } from '../../client/types'
import { useExportForm } from '../../composables/useExportForm'
import Modal from '../ui/Modal.vue'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'
import FormError from '../ui/FormError.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'

const props = defineProps<{
    table: Metadata
    exporting: boolean
    onExport: (format: ExportFormat, srid: number | null) => Promise<void>
}>()
const emit = defineEmits<{ close: [] }>()

const formats: { value: ExportFormat; label: string }[] = [
    { value: 'csv', label: 'CSV — colonnes seules, sans géométrie' },
    { value: 'geojson', label: 'GeoJSON — FeatureCollection' },
    { value: 'parquet', label: 'Parquet — colonnaire, compressé' }
]

const { format, srid, error, carriesGeometry, formatHint, confirm } = useExportForm(
    toRef(props, 'table'),
    props.onExport
)

async function confirmExport() {
    if (await confirm()) emit('close')
}
</script>

<template>
    <Modal title="Exporter la table" @close="emit('close')" @submit="confirmExport">
        <BaseSelect v-model="format" :options="formats" aria-label="Format" class="mb-1" />
        <p v-if="formatHint" class="mb-3 text-xs opacity-70">{{ formatHint }}</p>

        <BaseInput
            v-if="carriesGeometry"
            v-model="srid"
            label="Projection de sortie (code EPSG)"
            placeholder="2154, 3857, 4326…"
            class="font-mono"
        />
        <p v-if="carriesGeometry" class="mt-1 text-xs opacity-70">
            Laissez vide pour conserver la projection d’origine de la table. Le code EPSG choisi est
            ajouté au nom du fichier, car la plupart des formats ne l’indiquent pas de façon fiable.
        </p>

        <FormError :message="error" />

        <ModalFormActions
            :confirm-label="props.exporting ? 'Export en cours…' : 'Exporter'"
            :busy="props.exporting"
            :disabled="props.exporting"
            @cancel="emit('close')"
            @confirm="confirmExport"
        />
    </Modal>
</template>
