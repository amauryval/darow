import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { usePanes } from '@/darow/composables/usePanes'

describe('usePanes', () => {
    it('shows the table alone at first', () => {
        const { showMap, showTable } = usePanes(ref(true))
        expect([showMap.value, showTable.value]).toEqual([false, true])
    })

    it('shows the map beside the table, then alone', () => {
        const { showMap, showTable, split, toggleMap, toggleTable } = usePanes(ref(true))
        expect(split.value).toBe(false)
        toggleMap()
        expect([showMap.value, showTable.value, split.value]).toEqual([true, true, true])
        toggleTable()
        expect([showMap.value, showTable.value, split.value]).toEqual([true, false, false])
    })

    it('shows the table again when the map is hidden', () => {
        const { showMap, showTable, toggleMap, toggleTable } = usePanes(ref(true))
        toggleMap()
        toggleTable()
        toggleMap()
        expect([showMap.value, showTable.value]).toEqual([false, true])
    })

    it('shows the map in place of the table hidden alone', () => {
        const { showMap, showTable, toggleTable } = usePanes(ref(true))
        toggleTable()
        expect([showMap.value, showTable.value]).toEqual([true, false])
    })

    it('shows no map without a geometry', () => {
        const { showMap, showTable, toggleMap, toggleTable } = usePanes(ref(false))
        toggleMap()
        toggleTable()
        expect([showMap.value, showTable.value]).toEqual([false, true])
    })
})
