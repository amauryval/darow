import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useFeatureSelection } from './useFeatureSelection'
import { flush, withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import { getRowsByPkey } from '../client/tables'
import { getFilteredExtent } from '../client/geo'
import type { Row } from '../client/types'

vi.mock('../client/tables', () => ({ getRowsByPkey: vi.fn() }))
vi.mock('../client/geo', () => ({ getFilteredExtent: vi.fn() }))

const hydrate = vi.mocked(getRowsByPkey)
const extent = vi.mocked(getFilteredExtent)

function row(pkey: string): Row {
    return { pkey, name: pkey }
}

function setup() {
    const tableId = ref('t1')
    const { result } = withSetup(() => useFeatureSelection(tableId))
    return { tableId, ...result }
}

describe('useFeatureSelection', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        hydrate.mockImplementation(async (_tableId, pkeys) => ({ rows: pkeys.map(row) }))
        extent.mockResolvedValue({ bbox: '0,0,1,1' })
    })

    it('toggles a feature in and out of the selection', () => {
        const { selectedPkeys, toggle } = setup()

        toggle('p1')
        expect(selectedPkeys.value).toEqual(['p1'])

        toggle('p1')
        expect(selectedPkeys.value).toEqual([])
    })

    it('empties the selection on clear', () => {
        const { selectedPkeys, toggle, clear } = setup()
        toggle('p1')

        clear()

        expect(selectedPkeys.value).toEqual([])
    })

    it('hydrates the selected rows in one batched request', async () => {
        const { selectedPkeys, selectedRows } = setup()

        selectedPkeys.value = ['p1', 'p2']
        await nextTick()
        await flush()

        expect(hydrate).toHaveBeenCalledWith('t1', ['p1', 'p2'])
        expect(selectedRows.value.map((r) => r.pkey)).toEqual(['p1', 'p2'])
    })

    it('re-fetches only what the previous selection did not already hold', async () => {
        const { selectedPkeys } = setup()
        selectedPkeys.value = ['p1']
        await nextTick()
        await flush()

        selectedPkeys.value = ['p1', 'p2']
        await nextTick()
        await flush()

        expect(hydrate).toHaveBeenLastCalledWith('t1', ['p2'])
    })

    it('toasts a failed hydration and keeps what it could resolve', async () => {
        hydrate.mockRejectedValue(new Error('boom'))
        const { selectedPkeys, selectedRows } = setup()

        selectedPkeys.value = ['p1']
        await nextTick()
        await flush()

        expect(selectedRows.value).toEqual([])
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de charger les lignes sélectionnées : boom'
        })
    })

    it('drops a selection that would mean nothing in another table', async () => {
        const { tableId, selectedPkeys } = setup()
        selectedPkeys.value = ['p1']
        await nextTick()
        await flush()

        tableId.value = 't2'
        await nextTick()

        expect(selectedPkeys.value).toEqual([])
    })

    it('frames the selection through the filter DSL, as an OR of equalities', async () => {
        const { selectedPkeys, zoomToSelection, zoomBbox, zoomToken, zooming } = setup()
        selectedPkeys.value = ['p1', 'p2']
        await nextTick()
        await flush()

        await zoomToSelection()

        expect(extent).toHaveBeenCalledWith('t1', [
            {
                bool: 'OR',
                conditions: [
                    { column: '__pkey', operator: '=', value: 'p1' },
                    { column: '__pkey', operator: '=', value: 'p2' }
                ]
            }
        ])
        expect(zoomBbox.value).toBe('0,0,1,1')
        expect(zoomToken.value).toBe(1)
        expect(zooming.value).toBe(false)
    })

    it('bumps the token even on an unchanged bbox, so the map re-frames', async () => {
        const { selectedPkeys, zoomToSelection, zoomToken } = setup()
        selectedPkeys.value = ['p1']
        await nextTick()
        await flush()

        await zoomToSelection()
        await zoomToSelection()

        expect(zoomToken.value).toBe(2)
    })

    it('frames nothing on an empty selection', async () => {
        const { zoomToSelection } = setup()

        await zoomToSelection()

        expect(extent).not.toHaveBeenCalled()
    })

    it('toasts a failed framing', async () => {
        extent.mockRejectedValue(new Error('boom'))
        const { selectedPkeys, zoomToSelection, zooming } = setup()
        selectedPkeys.value = ['p1']
        await nextTick()
        await flush()

        await zoomToSelection()

        expect(zooming.value).toBe(false)
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de cadrer sur la sélection : boom'
        })
    })
})
