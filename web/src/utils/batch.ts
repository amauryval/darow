// How many requests a batched run keeps in flight at once. The server
// serializes row writes on its own write lock anyway (back §4.6), so one
// request at a time paid a round-trip per row for nothing; the cap keeps a
// large selection from opening a socket per row.
export const BATCH_SIZE = 8

// Runs `task` over `items` in parallel batches, in order, stopping at the
// first batch that holds a refusal. What already went through has gone
// through — it is named in `done`, and the caller reconciles on it rather than
// assuming all or nothing.
export async function inBatches<T>(items: T[], task: (item: T) => Promise<unknown>) {
    const done: T[] = []
    let reason: unknown = null
    for (let i = 0; i < items.length && reason === null; i += BATCH_SIZE) {
        const batch = items.slice(i, i + BATCH_SIZE)
        const outcomes = await Promise.allSettled(batch.map(task))
        outcomes.forEach((outcome, j) => {
            const item = batch[j]
            if (outcome.status === 'fulfilled') {
                if (item !== undefined) done.push(item)
            } else if (reason === null) {
                reason = outcome.reason ?? new Error('échec')
            }
        })
    }
    return { done, reason }
}
