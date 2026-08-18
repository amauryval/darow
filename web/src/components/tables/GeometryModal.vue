<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy } from '@lucide/vue'
import { excerpt, formatGeoJSON } from '../../utils/geometry'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import ErrorState from '../ui/ErrorState.vue'
import Skeleton from '../ui/Skeleton.vue'
import Typography from '../ui/Typography.vue'

// The geometry of a single row, in the two encodings it is read for. It is
// presentational: what is loaded, retried, reprojected or copied is decided by
// useRowGeometry in the panel above. The EPSG field changes what is displayed
// and nothing else — the stored geometry never moves.
const props = defineProps<{
    pkey: string
    geometry: GeoJSON.Geometry | null
    wkt: string
    // What the values below are actually expressed in, as the server resolved
    // it — never rebuilt from the field, which may hold a code not applied yet.
    crs: string
    // The table's own CRS: what the field opens on, so the row is first shown
    // in the CRS it was imported in.
    tableSrid?: number | null
    loading: boolean
    error: string
}>()
const emit = defineEmits<{
    close: []
    retry: []
    reproject: [srid: number | null]
    copy: [text: string, label: string]
}>()

const geoJSON = computed(() => formatGeoJSON(props.geometry))
const hasGeometry = computed(() => props.geometry !== null || props.wkt !== '')

const srid = ref(props.tableSrid ? String(props.tableSrid) : '')
const sridError = ref<string | null>(null)

function applyProjection() {
    const raw = srid.value.trim()
    if (!raw) {
        sridError.value = null
        emit('reproject', null)
        return
    }
    const code = Number(raw)
    if (!Number.isInteger(code) || code <= 0) {
        sridError.value = 'Le code EPSG doit être un entier positif.'
        return
    }
    sridError.value = null
    emit('reproject', code)
}
</script>

<template>
    <Modal title="Géométrie de la ligne" wide @close="emit('close')">
        <Typography variant="caption">
            Ligne {{ pkey }} — coordonnées en {{ crs || 'OGC:CRS84' }}.
        </Typography>

        <div class="mt-3 flex items-end gap-2">
            <BaseInput
                v-model="srid"
                label="Projection affichée (code EPSG)"
                placeholder="2154, 3857, 4326…"
                class="flex-1 font-mono"
                @keyup.enter="applyProjection"
            />
            <BaseButton variant="secondary" :disabled="loading" @click="applyProjection">
                Afficher
            </BaseButton>
        </div>
        <p class="mt-1 text-xs opacity-70">
            Laissez vide pour conserver la projection d’origine de la table. Rien n’est modifié en
            base : seul l’affichage ci-dessous change.
        </p>
        <p v-if="sridError" class="mt-1 text-xs text-error">{{ sridError }}</p>

        <Skeleton v-if="loading" :rows="6" />
        <ErrorState v-else-if="error" :message="error" @retry="emit('retry')" />
        <Typography v-else-if="!hasGeometry" variant="body-sm" class="mt-4 block">
            Cette ligne ne porte aucune géométrie.
        </Typography>

        <div v-else class="mt-4 flex flex-col gap-4">
            <section class="flex flex-col gap-2">
                <div class="flex items-center justify-between gap-2">
                    <Typography variant="eyebrow">WKT</Typography>
                    <BaseButton
                        size="xs"
                        variant="secondary"
                        :icon="Copy"
                        @click="emit('copy', wkt, 'WKT')"
                        >Copier</BaseButton
                    >
                </div>
                <pre
                    class="max-h-48 overflow-auto rounded-lg bg-base-200 p-3 text-xs whitespace-pre-wrap"
                    >{{ excerpt(wkt) }}</pre
                >
            </section>

            <section class="flex flex-col gap-2">
                <div class="flex items-center justify-between gap-2">
                    <Typography variant="eyebrow">GeoJSON</Typography>
                    <BaseButton
                        size="xs"
                        variant="secondary"
                        :icon="Copy"
                        @click="emit('copy', geoJSON, 'GeoJSON')"
                    >
                        Copier
                    </BaseButton>
                </div>
                <pre
                    class="max-h-64 overflow-auto rounded-lg bg-base-200 p-3 text-xs whitespace-pre-wrap"
                    >{{ excerpt(geoJSON) }}</pre
                >
            </section>
        </div>

        <div class="mt-4 flex justify-end">
            <BaseButton variant="secondary" @click="emit('close')">Fermer</BaseButton>
        </div>
    </Modal>
</template>
