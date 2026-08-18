import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useEditSession } from './useEditSession'
import { useToast } from './useToast'
import type { LayerChanges } from './useEditLayer'
import type { useLayerEditing } from './useLayerEditing'
import type { useRowEdits } from './useRowEdits'

function rowEdits(counts: { dirty?: number; drafts?: number; invalid?: number } = {}) {
    return {
        active: ref(false),
        dirtyCount: ref(counts.dirty ?? 0),
        draftCount: ref(counts.drafts ?? 0),
        invalidCount: ref(counts.invalid ?? 0),
        pendingUpdates: ref([{ pkey: 'p1', values: { name: 'Nantes' } }]),
        pendingInserts: ref([{ name: 'Nouvelle' }]),
        start: vi.fn(),
        stop: vi.fn()
    } as unknown as ReturnType<typeof useRowEdits>
}

function layerEditing(saved = true) {
    return {
        active: ref(false),
        start: vi.fn(),
        cancel: vi.fn(),
        save: vi.fn().mockResolvedValue(saved)
    } as unknown as ReturnType<typeof useLayerEditing>
}

function setup(
    counts: { dirty?: number; drafts?: number; invalid?: number } = {},
    isGeo = true,
    saved = true
) {
    const rows = rowEdits(counts)
    const layer = layerEditing(saved)
    return { rows, layer, ...useEditSession(rows, layer, ref(isGeo)) }
}

describe('useEditSession', () => {
    beforeEach(() => {
        useToast().toasts.value = []
    })

    it('opens both halves at once on a geo table', () => {
        const { rows, layer, toggle } = setup()

        toggle()

        expect(rows.start).toHaveBeenCalled()
        expect(layer.start).toHaveBeenCalled()
    })

    it('opens the grid alone on a table with no geometry', () => {
        const { rows, layer, toggle } = setup({}, false)

        toggle()

        expect(rows.start).toHaveBeenCalled()
        expect(layer.start).not.toHaveBeenCalled()
    })

    it('counts both halves of what one Annuler would drop', () => {
        const { unsavedCount, changedGeometries } = setup({ dirty: 2, drafts: 1 })

        changedGeometries.value = 3

        expect(unsavedCount.value).toBe(6)
    })

    it('leaves an empty session with no question to answer', () => {
        const { rows, layer, confirmingCancel, cancel } = setup()

        cancel()

        expect(confirmingCancel.value).toBe(false)
        expect(layer.cancel).toHaveBeenCalled()
        expect(rows.stop).toHaveBeenCalled()
    })

    it('asks before losing work', () => {
        const { rows, confirmingCancel, cancel } = setup({ dirty: 1 })

        cancel()

        expect(confirmingCancel.value).toBe(true)
        expect(rows.stop).not.toHaveBeenCalled()
    })

    it('drops both halves, and the geometry count, on discard', () => {
        const { rows, layer, changedGeometries, confirmingCancel, discard } = setup({ dirty: 1 })
        changedGeometries.value = 2
        confirmingCancel.value = true

        discard()

        expect(confirmingCancel.value).toBe(false)
        expect(changedGeometries.value).toBe(0)
        expect(layer.cancel).toHaveBeenCalled()
        expect(rows.stop).toHaveBeenCalled()
    })

    it('leaves the mode when the toggle is used on an open session', () => {
        const { rows, layer, toggle } = setup()
        vi.mocked(layer.active).value = true

        toggle()

        expect(rows.start).not.toHaveBeenCalled()
        expect(rows.stop).toHaveBeenCalled()
    })

    it('refuses a save the grid can point at, before writing anything', async () => {
        const { layer, save } = setup({ invalid: 2 })

        await save()

        expect(layer.save).not.toHaveBeenCalled()
        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message:
                "2 valeur(s) ne correspondent pas au type de leur colonne : corrigez-les avant d'enregistrer."
        })
    })

    it('writes the geometries and the cells together, then closes the mode', async () => {
        const changes: LayerChanges = { updates: new Map(), inserts: [] }
        const { rows, layer, changedGeometries, save } = setup()
        changedGeometries.value = 2

        await save(changes)

        expect(layer.save).toHaveBeenCalledWith(
            changes,
            [{ pkey: 'p1', values: { name: 'Nantes' } }],
            [{ name: 'Nouvelle' }]
        )
        expect(changedGeometries.value).toBe(0)
        expect(rows.stop).toHaveBeenCalled()
    })

    it('keeps the mode open when the write did not land', async () => {
        const { rows, changedGeometries, save } = setup({}, true, false)
        changedGeometries.value = 2

        await save()

        expect(changedGeometries.value).toBe(2)
        expect(rows.stop).not.toHaveBeenCalled()
    })
})
