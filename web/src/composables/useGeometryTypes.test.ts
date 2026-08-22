import { describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useGeometryTypes } from './useGeometryTypes'
import { withSetup, flush } from '../test/fixtures'
import { getGeometryTypes } from '../client/geo'

vi.mock('../client/geo', () => ({ getGeometryTypes: vi.fn() }))

const read = vi.mocked(getGeometryTypes)

function setup(active = true) {
    const tableId = ref('t1')
    const isActive = ref(active)
    const version = ref(0)
    const { result } = withSetup(() => useGeometryTypes(tableId, isActive, version))
    return { tableId, isActive, version, ...result }
}

describe('useGeometryTypes', () => {
    it('reads the table’s geometry types as soon as a session opens', async () => {
        read.mockResolvedValue({ types: ['POLYGON', 'MULTIPOLYGON'] })

        const { family } = setup()
        await flush()

        expect(read).toHaveBeenCalledWith('t1')
        expect(family.value).toBe('polygon')
    })

    it('reads nothing while no session is open', async () => {
        const { family } = setup(false)
        await flush()

        expect(read).not.toHaveBeenCalled()
        expect(family.value).toBeNull()
    })

    it('reads again after a write, which is what locks an empty table', async () => {
        read.mockResolvedValue({ types: [] })
        const { version, family } = setup()
        await flush()
        expect(family.value).toBeNull()

        read.mockResolvedValue({ types: ['POINT'] })
        version.value = 1
        await nextTick()
        await flush()

        expect(family.value).toBe('point')
    })

    it('reads again when the session moves to another table', async () => {
        read.mockResolvedValue({ types: ['POINT'] })
        const { tableId } = setup()
        await flush()

        tableId.value = 't2'
        await nextTick()
        await flush()

        expect(read).toHaveBeenLastCalledWith('t2')
    })

    it('leaves every tool armed when the read fails — the write is what says no', async () => {
        read.mockRejectedValue(new Error('boom'))

        const { family } = setup()
        await flush()

        expect(family.value).toBeNull()
    })
})
