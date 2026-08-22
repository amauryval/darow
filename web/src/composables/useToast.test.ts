import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useToast } from './useToast'

describe('useToast', () => {
    beforeEach(() => vi.useFakeTimers())

    afterEach(() => {
        const { toasts } = useToast()
        toasts.value = []
        vi.useRealTimers()
    })

    it('shares one queue across every caller', () => {
        useToast().success('table créée')

        expect(useToast().toasts.value).toHaveLength(1)
    })

    it('queues a toast with its kind and message', () => {
        const { toasts, error } = useToast()

        error('la table n’existe pas')

        expect(toasts.value[0]).toMatchObject({ kind: 'error', message: 'la table n’existe pas' })
    })

    it('gives each toast its own identifier', () => {
        const { toasts, success } = useToast()

        success('un')
        success('deux')

        expect(toasts.value[0].id).not.toBe(toasts.value[1].id)
    })

    it('dismisses a success on its own after four seconds', () => {
        const { toasts, success } = useToast()
        success('table créée')

        vi.advanceTimersByTime(4000)

        expect(toasts.value).toHaveLength(0)
    })

    it('keeps an error until it is dismissed by hand', () => {
        const { toasts, error, dismiss } = useToast()
        error('échec du chargement')

        vi.advanceTimersByTime(60_000)
        expect(toasts.value).toHaveLength(1)

        dismiss(toasts.value[0].id)
        expect(toasts.value).toHaveLength(0)
    })

    it('dismisses only the toast named', () => {
        const { toasts, error } = useToast()
        error('un')
        error('deux')

        useToast().dismiss(toasts.value[0].id)

        expect(toasts.value.map((t) => t.message)).toEqual(['deux'])
    })
})
