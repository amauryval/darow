import { describe, expect, it, vi } from 'vitest'
import { BATCH_SIZE, inBatches } from './batch'

describe('inBatches', () => {
    it('runs every item and names them all as done', async () => {
        const items = [1, 2, 3]
        const task = vi.fn().mockResolvedValue(undefined)

        const { done, reason } = await inBatches(items, task)

        expect(task).toHaveBeenCalledTimes(3)
        expect(done).toEqual(items)
        expect(reason).toBeNull()
    })

    it('keeps at most BATCH_SIZE requests in flight', async () => {
        let inFlight = 0
        let peak = 0
        const items = Array.from({ length: BATCH_SIZE * 2 + 3 }, (_, i) => i)

        await inBatches(items, async () => {
            inFlight += 1
            peak = Math.max(peak, inFlight)
            await Promise.resolve()
            inFlight -= 1
        })

        expect(peak).toBe(BATCH_SIZE)
    })

    it('stops after the batch holding a refusal and reports what went through', async () => {
        const items = Array.from({ length: BATCH_SIZE + 2 }, (_, i) => i)
        const boom = new Error('refus')
        const task = vi.fn(async (item: number) => {
            if (item === 1) throw boom
        })

        const { done, reason } = await inBatches(items, task)

        expect(reason).toBe(boom)
        expect(done).toEqual(items.slice(0, BATCH_SIZE).filter((i) => i !== 1))
        expect(task).toHaveBeenCalledTimes(BATCH_SIZE)
    })

    it('keeps the first refusal when a batch holds several', async () => {
        const first = new Error('premier')
        const { reason } = await inBatches([0, 1], async (item) => {
            throw item === 0 ? first : new Error('second')
        })

        expect(reason).toBe(first)
    })

    it('substitutes an error for a rejection carrying no reason', async () => {
        const { reason } = await inBatches([0], () => Promise.reject())

        expect(reason).toBeInstanceOf(Error)
    })

    it('does nothing on an empty list', async () => {
        const task = vi.fn()

        await expect(inBatches([], task)).resolves.toEqual({ done: [], reason: null })
        expect(task).not.toHaveBeenCalled()
    })
})
