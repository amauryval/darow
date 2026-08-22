import { ref } from 'vue'
import { getSourceMetadata, loadSource } from '../client/sources'
import type { LoadOptions } from '../client/sources'
import type { Source } from '../client/types'
import { useAsyncAction } from './useAsyncAction'
import { layerKey } from './useSources'
import { useToast } from './useToast'

// The source a load form is open on: a plain file, or one layer of a
// multi-layer one.
export interface LoadTarget {
    source: Source
    layerName?: string
}

// The load flow of one source: pick it, read the CRS it declares, then confirm
// — a new table, or a reload of an existing one in place.
export function useSourceLoad() {
    const toast = useToast()

    const loadTarget = ref<LoadTarget | null>(null)
    const detectedSrid = ref('')
    // Whether the source being loaded has a geometry to project. Until the
    // detection answers, the question is assumed to apply — asking and finding
    // out it did not costs nothing, loading without asking cannot be undone.
    const epsgApplies = ref(true)
    const { isLoading: epsgDetecting, error: detectError, run: runDetect } = useAsyncAction()
    const { isLoading: loading, error: loadingError, run: runLoad } = useAsyncAction()

    // Opens the load form on this source and fills in the CRS it declares; the
    // form stays open either way, the name of the table being asked too.
    async function startLoad(source: Source, layerName?: string) {
        const key = layerName ? layerKey(source, layerName) : source.name
        loadTarget.value = { source, layerName }
        detectedSrid.value = ''
        epsgApplies.value = true
        const crs = await runDetect(
            () => getSourceMetadata(source.name, layerName),
            `Impossible de détecter le système de coordonnées de ${key}`
        )
        if (crs === null) {
            toast.error(detectError.value)
            return
        }
        if (targetKey() === key) {
            detectedSrid.value = crs.srid ? String(crs.srid) : ''
            epsgApplies.value = crs.hasGeometry
        }
    }

    function targetKey(): string | null {
        const target = loadTarget.value
        if (!target) return null
        return target.layerName ? layerKey(target.source, target.layerName) : target.source.name
    }

    function cancelLoad() {
        loadTarget.value = null
        detectedSrid.value = ''
        epsgApplies.value = true
    }

    // Creates the table, or replaces an existing one's data when the form
    // named it. Reports whether the write landed, so the caller can refresh
    // what it holds on the tables that now exist.
    async function confirmLoad(options: LoadOptions): Promise<boolean> {
        const target = loadTarget.value
        const key = targetKey()
        if (!target || !key) return false

        const result = await runLoad(
            () => loadSource(target.source.name, { ...options, layer: target.layerName }),
            `Impossible de charger ${key}`
        )
        if (result === null) {
            toast.error(loadingError.value)
            return false
        }
        loadTarget.value = null
        toast.success(
            options.replaceTableId
                ? `${key} rechargé — la table a été mise à jour.`
                : `${key} chargé — la table a été créée.`
        )
        return true
    }

    return {
        loadTarget,
        detectedSrid,
        epsgDetecting,
        epsgApplies,
        loading,
        startLoad,
        cancelLoad,
        confirmLoad
    }
}
