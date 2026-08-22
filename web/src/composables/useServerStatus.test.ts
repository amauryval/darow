import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { useServerStatus } from './useServerStatus'
import { withSetup } from '../test/fixtures'
import { readyz } from '../client/ops'

vi.mock('../client/ops', () => ({ readyz: vi.fn() }))

const probe = vi.mocked(readyz)

describe('useServerStatus', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('starts as checking, before the first probe answers', () => {
        probe.mockReturnValue(new Promise(() => {}))

        const { result } = withSetup(() => useServerStatus())

        expect(result.status.value).toBe('checking')
    })

    it('reports the server as operational once readyz answers', async () => {
        probe.mockResolvedValue(undefined)

        const { result } = withSetup(() => useServerStatus())
        await nextTick()

        expect(result.status.value).toBe('ok')
    })

    it('reports it as unavailable when readyz refuses', async () => {
        probe.mockRejectedValue(new Error('down'))

        const { result } = withSetup(() => useServerStatus())
        await nextTick()

        expect(result.status.value).toBe('down')
    })

    it('probes again every twenty seconds', async () => {
        probe.mockResolvedValue(undefined)
        withSetup(() => useServerStatus())

        await vi.advanceTimersByTimeAsync(40_000)

        expect(probe).toHaveBeenCalledTimes(3)
    })

    it('clears its interval on unmount', async () => {
        probe.mockResolvedValue(undefined)
        const { unmount } = withSetup(() => useServerStatus())

        unmount()
        await vi.advanceTimersByTimeAsync(60_000)

        expect(probe).toHaveBeenCalledTimes(1)
    })

    it('names each status in French', () => {
        probe.mockResolvedValue(undefined)

        const { result } = withSetup(() => useServerStatus())

        expect(result.label).toEqual({
            checking: 'Vérification…',
            ok: 'Opérationnel',
            down: 'Indisponible'
        })
    })
})
