import { computed, ref, watch, type Ref } from 'vue'
import { getGeometryTypes } from '../client/geo'
import { lockedFamily, type GeometryFamily } from '../utils/geometryFamilies'

// The family a table's geometry column is locked to, read when an edit
// session opens and again after every write — drawing the first shape into an
// empty table is what locks it.
//
// Best-effort on purpose: the answer only removes tools the server would
// refuse anyway, so a failed read leaves all three armed and the write is
// what says no (§8.2).
export function useGeometryTypes(tableId: Ref<string>, active: Ref<boolean>, version: Ref<number>) {
    const types = ref<string[]>([])
    const family = computed<GeometryFamily | null>(() => lockedFamily(types.value))

    async function load() {
        if (!active.value) return
        try {
            types.value = (await getGeometryTypes(tableId.value)).types
        } catch {
            types.value = []
        }
    }

    watch([tableId, active, version], load, { immediate: true })

    return { family }
}
