import { describe, expect, it, vi } from 'vitest'
import { useAsyncAction } from './useAsyncAction'
import { ApiError } from '../client/http'

describe('useAsyncAction', () => {
    it('starts idle, with no error', () => {
        const { isLoading, error } = useAsyncAction()

        expect(isLoading.value).toBe(false)
        expect(error.value).toBe('')
    })

    it('raises the loading flag for the attempt and lowers it after', async () => {
        const { isLoading, run } = useAsyncAction()
        const seen: boolean[] = []

        await run(async () => seen.push(isLoading.value), 'échec')

        expect(seen).toEqual([true])
        expect(isLoading.value).toBe(false)
    })

    it('answers the action’s own result', async () => {
        const { run } = useAsyncAction()

        await expect(run(async () => ({ id: 't1' }), 'échec')).resolves.toEqual({ id: 't1' })
    })

    it('answers null and states the failure in French', async () => {
        const { error, run } = useAsyncAction()

        const result = await run(() => Promise.reject(new Error('boom')), 'Impossible de dériver')

        expect(result).toBeNull()
        expect(error.value).toBe('Impossible de dériver : boom')
    })

    it('prefers the server’s own message over the fallback', async () => {
        const { error, run } = useAsyncAction()

        await run(
            () =>
                Promise.reject(
                    new ApiError({
                        error: 'conflict',
                        message: 'nom déjà pris',
                        code: 409,
                        requestId: 'r1'
                    })
                ),
            'Impossible de dériver'
        )

        expect(error.value).toContain('nom déjà pris')
    })

    it('clears the previous error before a new attempt', async () => {
        const { error, run } = useAsyncAction()
        await run(() => Promise.reject(new Error('boom')), 'échec')

        await run(async () => 'ok', 'échec')

        expect(error.value).toBe('')
    })

    it('lowers the loading flag even when the action throws', async () => {
        const { isLoading, run } = useAsyncAction()

        await run(() => Promise.reject(new Error('boom')), 'échec')

        expect(isLoading.value).toBe(false)
    })

    it('leaves the loading flag down when the action is not called twice at once', async () => {
        const { run } = useAsyncAction()
        const action = vi.fn().mockResolvedValue('ok')

        await run(action, 'échec')

        expect(action).toHaveBeenCalledTimes(1)
    })
})
