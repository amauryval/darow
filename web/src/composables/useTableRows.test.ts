import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useTableRows } from './useTableRows'
import { flush, stubIntersectionObserver, withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import { addRows, deleteRows, getRowGeometry, listRows } from '../client/tables'
import type { Row } from '../client/types'

vi.mock('../client/tables', () => ({
    listRows: vi.fn(),
    addRows: vi.fn(),
    deleteRows: vi.fn(),
    getRowGeometry: vi.fn()
}))

const list = vi.mocked(listRows)
const add = vi.mocked(addRows)
const remove = vi.mocked(deleteRows)
const geometry = vi.mocked(getRowGeometry)

function row(pkey: string, name: string): Row {
    return { pkey, name }
}

async function setup(geometryColumn?: string) {
    const onMutated = vi.fn()
    const { result } = withSetup(() =>
        useTableRows(ref('t1'), ref(['name']), ref(geometryColumn), onMutated, ref(100))
    )
    await flush()
    return { onMutated, ...result }
}

describe('useTableRows', () => {
    beforeEach(() => {
        stubIntersectionObserver()
        useToast().toasts.value = []
        list.mockResolvedValue({
            rows: [row('p1', 'Nantes'), row('p2', 'Rennes')],
            total: 2,
            limit: 100,
            offset: 0
        })
        add.mockResolvedValue({ pkeys: ['p9'] })
        remove.mockResolvedValue({ pkeys: ['p1'], rowsRemoved: 1 })
    })

    it('loads the page at the table’s effective row limit', async () => {
        const { rows, total } = await setup()

        expect(list).toHaveBeenCalledWith('t1', 0, 100, '')
        expect(rows.value).toHaveLength(2)
        expect(total.value).toBe(2)
    })

    it('deletes the whole selection in one request and drops exactly those rows', async () => {
        const { rows, total, deleteRows: removeSelection, onMutated } = await setup()
        const { toasts } = useToast()

        await removeSelection(['p1'])

        expect(remove).toHaveBeenCalledWith('t1', ['p1'])
        expect(rows.value.map((r) => r.pkey)).toEqual(['p2'])
        expect(total.value).toBe(1)
        expect(list).toHaveBeenCalledTimes(1)
        expect(onMutated).toHaveBeenCalled()
        expect(toasts.value[0].message).toBe('Ligne supprimée.')
    })

    it('counts the rows it deleted in the confirmation', async () => {
        const { deleteRows: removeSelection } = await setup()

        await removeSelection(['p1', 'p2'])

        expect(useToast().toasts.value[0].message).toBe('2 lignes supprimées.')
    })

    it('writes nothing on an empty selection', async () => {
        const { deleteRows: removeSelection } = await setup()

        await removeSelection([])

        expect(remove).not.toHaveBeenCalled()
    })

    it('toasts a refused deletion and keeps the rows', async () => {
        remove.mockRejectedValue(new Error('boom'))
        const { rows, deleteRows: removeSelection, onMutated } = await setup()

        await removeSelection(['p1'])

        expect(rows.value).toHaveLength(2)
        expect(onMutated).not.toHaveBeenCalled()
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de supprimer la sélection : boom'
        })
    })

    it('duplicates the attributes of the selected rows and re-reads the page', async () => {
        const { duplicateRows, onMutated } = await setup()

        await duplicateRows([row('p1', 'Nantes')])

        expect(add).toHaveBeenCalledWith('t1', [{ name: 'Nantes' }])
        expect(list).toHaveBeenCalledTimes(2)
        expect(onMutated).toHaveBeenCalled()
        expect(useToast().toasts.value[0].message).toBe('Ligne dupliquée.')
    })

    it('reads each geometry, which no row listing carries', async () => {
        geometry.mockResolvedValue({
            pkey: 'p1',
            geometry: { type: 'Point', coordinates: [1, 2] },
            wkt: '',
            crs: ''
        })
        const { duplicateRows } = await setup('geom')

        await duplicateRows([row('p1', 'Nantes')])

        expect(geometry).toHaveBeenCalledWith('t1', 'p1')
        expect(add).toHaveBeenCalledWith('t1', [
            { name: 'Nantes', geom: { type: 'Point', coordinates: [1, 2] } }
        ])
    })

    it('writes no copy when a geometry could not be read — none would land empty', async () => {
        geometry.mockRejectedValue(new Error('illisible'))
        const { duplicateRows } = await setup('geom')

        await duplicateRows([row('p1', 'Nantes')])

        expect(add).not.toHaveBeenCalled()
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de dupliquer la sélection : illisible'
        })
    })

    it('counts the copies in the confirmation', async () => {
        const { duplicateRows } = await setup()

        await duplicateRows([row('p1', 'Nantes'), row('p2', 'Rennes')])

        expect(useToast().toasts.value[0].message).toBe('2 lignes dupliquées.')
    })

    it('duplicates nothing on an empty selection', async () => {
        const { duplicateRows } = await setup()

        await duplicateRows([])

        expect(add).not.toHaveBeenCalled()
    })
})
