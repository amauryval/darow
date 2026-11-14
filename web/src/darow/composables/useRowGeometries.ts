import { useExactGeometries } from '@myapplibs/web-platform/components/map/useExactGeometries'
import * as api from '@/darow/api/darow'
import { geometryOf } from '@/darow/utils/geometry'
import { TABLE_LAYERS } from '@/darow/utils/tableLayers'

// useRowGeometries gives the exact geometries of the open table's features, read from their rows.
export function useRowGeometries(table: () => string) {
    return useExactGeometries(TABLE_LAYERS.layers, async (id) =>
        geometryOf(await api.getRow(table(), id))
    )
}
