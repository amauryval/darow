import { describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useColumnTransform } from './useColumnTransform'
import { previewColumnTransform, transformColumn } from '../client/tables'
import { createLeaf } from '../utils/filterTree'
import type { ColumnSchema } from '../client/types'

vi.mock('../client/tables', () => ({
    previewColumnTransform: vi.fn(),
    transformColumn: vi.fn()
}))

const preview = vi.mocked(previewColumnTransform)
const write = vi.mocked(transformColumn)

const columns: ColumnSchema[] = [
    { name: 'name', type: 'VARCHAR' },
    { name: 'height', type: 'INTEGER' }
]

function setup() {
    return useColumnTransform(ref('t1'), ref('height'), ref(columns))
}

describe('useColumnTransform', () => {
    it('blocks on an empty expression — a column update is one value, expressed once', () => {
        const { expression, incomplete } = setup()

        expect(incomplete.value).toBe(true)

        expression.value = '  '
        expect(incomplete.value).toBe(true)

        expression.value = 'height * 2'
        expect(incomplete.value).toBe(false)
    })

    it('previews the trimmed expression over the rows the conditions name', async () => {
        preview.mockResolvedValue({ matched: 3, sample: [{ pkey: 'p1', before: 1, after: 2 }] })
        const { expression, conditions, runPreview, previewMatched, previewSample } = setup()
        expression.value = ' height * 2 '
        conditions.value = [{ ...createLeaf('height'), operator: '>', value: '10' }]

        await runPreview()

        expect(preview).toHaveBeenCalledWith('t1', 'height', {
            cases: [],
            default: { kind: 'expression', expression: 'height * 2' },
            conditions: [{ column: 'height', operator: '>', value: 10, not: undefined }]
        })
        expect(previewMatched.value).toBe(3)
        expect(previewSample.value).toEqual([{ pkey: 'p1', before: 1, after: 2 }])
    })

    it('drops the preview as soon as the spec is edited — a stale one must not be confirmed', async () => {
        preview.mockResolvedValue({ matched: 3, sample: [] })
        const { expression, runPreview, previewMatched } = setup()
        expression.value = 'height * 2'
        await runPreview()

        expression.value = 'height * 3'
        await nextTick()

        expect(previewMatched.value).toBeNull()
    })

    it('states a refused preview in French, keeping its own error apart', async () => {
        preview.mockRejectedValue(new Error('expression invalide'))
        const { expression, runPreview, previewError, previewMatched, error } = setup()
        expression.value = 'nawak('

        await runPreview()

        expect(previewError.value).toBe(
            'Impossible de prévisualiser la mise à jour : expression invalide'
        )
        expect(previewMatched.value).toBeNull()
        expect(error.value).toBe('')
    })

    it('writes the same spec the preview answered for', async () => {
        write.mockResolvedValue({ written: 3 })
        const { expression, submit } = setup()
        expression.value = 'height * 2'

        await expect(submit()).resolves.toEqual({ written: 3 })

        expect(write).toHaveBeenCalledWith('t1', 'height', {
            cases: [],
            default: { kind: 'expression', expression: 'height * 2' },
            conditions: []
        })
    })

    it('states a refused write in French', async () => {
        write.mockRejectedValue(new Error('colonne en lecture seule'))
        const { expression, error, submit } = setup()
        expression.value = 'height * 2'

        await expect(submit()).resolves.toBeNull()
        expect(error.value).toBe(
            'Impossible de mettre à jour la colonne : colonne en lecture seule'
        )
    })
})
