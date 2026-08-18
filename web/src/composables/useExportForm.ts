import { computed, ref, type Ref } from 'vue'
import type { ExportFormat, Metadata } from '../client/types'
import { useAsyncAction } from './useAsyncAction'

// The draft and the write behind ExportModal: the format, the output CRS and
// their derived hints, plus confirm() wrapping onExport — the caller's own
// download call, passed in rather than made here. exporting (the busy flag
// TableHeader also reads) stays the caller's, so this owns only the form's
// own validation error and the mutation's.
export function useExportForm(
    table: Ref<Metadata>,
    onExport: (format: ExportFormat, srid: number | null) => Promise<void>
) {
    const format = ref<ExportFormat>(table.value.isGeo ? 'geojson' : 'csv')
    // Pre-filled with the table's own CRS, which is also what the server
    // falls back to: leaving the field untouched exports the table as it
    // came in.
    const srid = ref(table.value.srid ? String(table.value.srid) : '')
    const { error, run } = useAsyncAction()

    // CSV carries no geometry (back rule 7), so nothing is reprojected for it.
    const carriesGeometry = computed(() => table.value.isGeo && format.value !== 'csv')

    const parsedSrid = computed(() => {
        const raw = srid.value.trim()
        if (!raw) return null
        const code = Number(raw)
        return Number.isInteger(code) && code > 0 ? code : NaN
    })

    // Parquet becomes GeoParquet by itself on a geometry column — same file,
    // same extension, geo metadata in addition.
    const formatHint = computed(() => {
        if (format.value === 'parquet' && table.value.isGeo) {
            return 'La géométrie fait de ce Parquet un GeoParquet.'
        }
        if (format.value === 'geojson' && !table.value.isGeo) {
            return 'Cette table n’a pas de géométrie : le GeoJSON sera refusé.'
        }
        return null
    })

    async function confirm(): Promise<boolean> {
        if (Number.isNaN(parsedSrid.value)) {
            error.value = 'Le code EPSG doit être un entier positif.'
            return false
        }
        const result = await run(
            () => onExport(format.value, carriesGeometry.value ? parsedSrid.value : null),
            "Impossible d'exporter la table"
        )
        return result !== null
    }

    return { format, srid, error, carriesGeometry, parsedSrid, formatHint, confirm }
}
