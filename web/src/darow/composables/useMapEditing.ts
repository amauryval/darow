import { computed, ref, shallowRef, watch } from 'vue'
import type { Geometry } from 'geojson'
import type { SnapFeature } from '@myapplibs/web-platform/components/map/topology'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useConfirm } from '@myapplibs/web-platform/components/composables/useConfirm'
import * as api from '@/darow/api/darow'
import { geometryOf, sameGeometry } from '@/darow/utils/geometry'
import { rowIdOf } from '@/darow/utils/rowValues'
import { useTable } from './useTable'

// useMapEditing edits the geometries on the map: the feature in hand, its changed neighbours, both
// hidden from the tiles; click takes a feature in hand while editing, unless the editor uses it.
export function useMapEditing() {
    const table = useTable()
    const { attempt } = useAttempt()
    const { confirmDialog } = useConfirm()

    const editing = ref(false)
    const rowId = ref<number | null>(null)
    const geometry = ref<Geometry | null>(null)
    const original = shallowRef<Geometry | null>(null)
    const neighbours = ref<SnapFeature[]>([])
    const busy = ref(false)
    const dirty = computed(
        () => !sameGeometry(geometry.value, original.value) || neighbours.value.length > 0
    )

    const load = (id: number | null, next: Geometry | null) => {
        rowId.value = id
        original.value = next
        geometry.value = next
        neighbours.value = []
    }

    const discard = async () => !dirty.value || confirmDialog(t('darow.mapEdit.confirmDiscard'))

    const edit = async (id: number) => {
        if (id === rowId.value || !(await discard())) return false
        return attempt(t('darow.row.loadFailed'), async () => {
            load(id, geometryOf(await api.getRow(table.name.value, id)))
        })
    }

    const create = async () => {
        if (await discard()) load(null, null)
    }

    const save = () =>
        attempt(
            t('darow.row.saveFailed'),
            async () => {
                const id = rowId.value === null ? {} : { id: rowId.value }
                const { ids } = await api.writeRows(table.name.value, {
                    rows: [
                        { ...id, values: {}, geometry: geometry.value },
                        ...neighbours.value.map((neighbour) => ({
                            id: Number(neighbour.id),
                            values: {},
                            geometry: neighbour.geometry
                        }))
                    ]
                })
                if (rowId.value === null) {
                    const inserted = await api.getRow(table.name.value, ids[0] ?? 0)
                    load(rowIdOf(inserted), geometryOf(inserted))
                    table.selectRow(inserted)
                } else {
                    original.value = geometry.value
                    neighbours.value = []
                }
                await table.changed()
                return t('darow.row.saved')
            },
            busy
        )

    const click = (id: number, editorUsesIt: boolean) => {
        if (!editing.value) return false
        if (!editorUsesIt) void edit(id)
        return true
    }

    const toggle = async () => {
        if (editing.value && !(await discard())) return
        load(null, null)
        editing.value = !editing.value
    }

    watch(table.name, () => {
        editing.value = false
        load(null, null)
    })

    return {
        editing,
        rowId,
        geometry,
        neighbours,
        busy,
        dirty,
        hidden: computed(() => [
            ...(rowId.value === null ? [] : [rowId.value]),
            ...neighbours.value.map((neighbour) => Number(neighbour.id))
        ]),
        status: computed(() =>
            rowId.value === null
                ? t('darow.mapEdit.drawing')
                : t('darow.mapEdit.editing', { id: String(rowId.value) })
        ),
        edit,
        click,
        create,
        save,
        cancel: () => load(rowId.value, original.value),
        toggle
    }
}
