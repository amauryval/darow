import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useLayerEditing } from './useLayerEditing'
import { withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import { addRows, updateRows } from '../client/tables'
import type { LayerChanges } from './useEditLayer'

vi.mock('../client/tables', () => ({
    addRows: vi.fn(),
    updateRows: vi.fn(),
    getRowGeometry: vi.fn()
}))
vi.mock('../client/geo', () => ({ getNeighbours: vi.fn() }))

const insert = vi.mocked(addRows)
const update = vi.mocked(updateRows)

const point: GeoJSON.Geometry = { type: 'Point', coordinates: [1, 2] }

const nothing: LayerChanges = { updates: new Map(), inserts: [] }

// `null` stands for a table with no geometry column at all.
function setup(geometryColumn: string | null = 'geom') {
    const tableId = ref('t1')
    const onMutated = vi.fn()
    const column = ref(geometryColumn === null ? undefined : geometryColumn)
    const { result } = withSetup(() => useLayerEditing(tableId, column, onMutated))
    return { tableId, onMutated, ...result }
}

describe('useLayerEditing', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        insert.mockResolvedValue({ pkeys: ['p9'] })
        update.mockResolvedValue({ rows: [{ pkey: 'p1' }] })
    })

    it('opens the session on a clean slate', () => {
        const { active, start, features } = setup()

        start()

        expect(active.value).toBe(true)
        expect(features.value).toEqual([])
    })

    it('writes both halves of a row in one request', async () => {
        const { start, save } = setup()
        start()

        await expect(
            save({ updates: new Map([['p1', point]]), inserts: [] }, [
                { pkey: 'p1', values: { name: 'Nantes' } }
            ])
        ).resolves.toBe(true)

        expect(update).toHaveBeenCalledWith('t1', [
            { pkey: 'p1', values: { name: 'Nantes', geom: point } }
        ])
    })

    it('pairs the nth drawn shape with the nth draft row', async () => {
        const { start, save } = setup()
        start()

        await save({ updates: new Map(), inserts: [point] }, [], [{ name: 'Nouvelle' }])

        expect(insert).toHaveBeenCalledWith('t1', [{ name: 'Nouvelle', geom: point }])
    })

    it('inserts a draft with no shape as it stands', async () => {
        const { start, save } = setup(null)
        start()

        await save(nothing, [], [{ name: 'Nouvelle' }])

        expect(insert).toHaveBeenCalledWith('t1', [{ name: 'Nouvelle' }])
    })

    it('refuses a geometry on a table that has no column to hold it', async () => {
        const { start, save } = setup(null)
        start()

        await expect(save({ updates: new Map([['p1', point]]), inserts: [] })).resolves.toBe(false)

        expect(update).not.toHaveBeenCalled()
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: "Cette table n'a pas de colonne géométrique : rien ne peut être enregistré."
        })
    })

    it('closes an empty session without writing anything', async () => {
        const { start, save, active, onMutated } = setup()
        start()

        await expect(save(nothing)).resolves.toBe(true)

        expect(update).not.toHaveBeenCalled()
        expect(insert).not.toHaveBeenCalled()
        expect(active.value).toBe(false)
        expect(onMutated).not.toHaveBeenCalled()
    })

    it('states what the save wrote, and closes the session', async () => {
        const { start, save, active, onMutated, saving } = setup()
        start()

        await save({ updates: new Map([['p1', point]]), inserts: [point] }, [], [{ name: 'x' }])

        expect(useToast().toasts.value[0].message).toBe(
            '1 géométrie(s) mise(s) à jour, 1 ligne(s) ajoutée(s).'
        )
        expect(active.value).toBe(false)
        expect(saving.value).toBe(false)
        expect(onMutated).toHaveBeenCalled()
    })

    it('keeps the session open when the write is refused', async () => {
        update.mockRejectedValue(new Error('géométrie invalide'))
        const { start, save, active, onMutated } = setup()
        start()

        await expect(save({ updates: new Map([['p1', point]]), inserts: [] })).resolves.toBe(false)

        expect(active.value).toBe(true)
        expect(onMutated).not.toHaveBeenCalled()
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: "Impossible d'enregistrer les modifications : géométrie invalide"
        })
    })

    it('drops the session on cancel', () => {
        const { start, cancel, active } = setup()
        start()

        cancel()

        expect(active.value).toBe(false)
    })

    it('drops a session whose pkeys mean nothing in another table', async () => {
        const { tableId, start, active } = setup()
        start()

        tableId.value = 't2'
        await nextTick()

        expect(active.value).toBe(false)
    })
})
