<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ExportFormat, Metadata } from '../../client/types'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'

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

const format = ref<ExportFormat>(props.table.isGeo ? 'geojson' : 'csv')
// Pre-filled with the table's own CRS, which is also what the server falls
// back to: leaving the field untouched exports the table as it came in.
const srid = ref(props.table.srid ? String(props.table.srid) : '')
const error = ref<string | null>(null)

// CSV carries no geometry (back rule 7), so nothing is reprojected for it.
const carriesGeometry = computed(() => props.table.isGeo && format.value !== 'csv')

const parsedSrid = computed(() => {
    const raw = srid.value.trim()
    if (!raw) return null
    const code = Number(raw)
    return Number.isInteger(code) && code > 0 ? code : NaN
})

// Parquet becomes GeoParquet by itself on a geometry column — same file,
// same extension, geo metadata in addition.
const formatHint = computed(() => {
    if (format.value === 'parquet' && props.table.isGeo) {
        return 'La géométrie fait de ce Parquet un GeoParquet.'
    }
    if (format.value === 'geojson' && !props.table.isGeo) {
        return 'Cette table n’a pas de géométrie : le GeoJSON sera refusé.'
    }
    return null
})

async function confirmExport() {
    if (Number.isNaN(parsedSrid.value)) {
        error.value = 'Le code EPSG doit être un entier positif.'
        return
    }
    error.value = null
    try {
        await props.onExport(format.value, carriesGeometry.value ? parsedSrid.value : null)
        emit('close')
    } catch (e) {
        error.value = e instanceof Error ? e.message : "Impossible d'exporter la table"
    }
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

        <p v-if="error" class="mt-3 text-xs text-error">{{ error }}</p>

        <div class="mt-4 flex justify-end gap-2">
            <BaseButton variant="secondary" @click="emit('close')">Annuler</BaseButton>
            <BaseButton :disabled="props.exporting" @click="confirmExport">
                {{ props.exporting ? 'Export en cours…' : 'Exporter' }}
            </BaseButton>
        </div>
    </Modal>
</template>
