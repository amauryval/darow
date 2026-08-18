import { describe, expect, it } from 'vitest'
import { useComputedColumnList } from './useComputedColumnList'

describe('useComputedColumnList', () => {
    it('starts empty', () => {
        const { items, normalized, incomplete } = useComputedColumnList()

        expect(items.value).toEqual([])
        expect(normalized.value).toEqual([])
        expect(incomplete.value).toBe(false)
    })

    it('adds an empty entry', () => {
        const { items, add } = useComputedColumnList()

        add()

        expect(items.value).toEqual([{ alias: '', expression: '' }])
    })

    it('patches one entry by index, leaving the others alone', () => {
        const { items, add, update } = useComputedColumnList()
        add()
        add()

        update(1, { alias: 'surface' })

        expect(items.value).toEqual([
            { alias: '', expression: '' },
            { alias: 'surface', expression: '' }
        ])
    })

    it('removes an entry by index', () => {
        const { items, add, update, remove } = useComputedColumnList()
        add()
        add()
        update(0, { alias: 'a' })

        remove(0)

        expect(items.value).toEqual([{ alias: '', expression: '' }])
    })

    it('trims what a submitted spec sends', () => {
        const { add, update, normalized } = useComputedColumnList()
        add()

        update(0, { alias: '  surface ', expression: ' ST_Area(geom) ' })

        expect(normalized.value).toEqual([{ alias: 'surface', expression: 'ST_Area(geom)' }])
    })

    it('drops an entry left wholly empty rather than sending a blank pair', () => {
        const { add, update, normalized } = useComputedColumnList()
        add()
        add()
        update(0, { alias: 'surface', expression: 'ST_Area(geom)' })

        expect(normalized.value).toHaveLength(1)
    })

    it('reports a half-filled entry, which the server would refuse', () => {
        const { add, update, incomplete } = useComputedColumnList()
        add()

        update(0, { alias: 'surface' })
        expect(incomplete.value).toBe(true)

        update(0, { expression: 'ST_Area(geom)' })
        expect(incomplete.value).toBe(false)
    })

    it('does not report an entry left wholly empty as half filled', () => {
        const { add, incomplete } = useComputedColumnList()

        add()

        expect(incomplete.value).toBe(false)
    })
})
