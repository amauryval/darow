import { describe, expect, it, vi } from 'vitest'
import { computed, ref } from 'vue'
import { useWriteGuard } from './useWriteGuard'
import { useSession } from './useSession'

vi.mock('./useSession', () => ({ useSession: vi.fn() }))

function session(authenticated: boolean, canWrite: boolean, canManage: boolean) {
    const state = {
        authenticated: ref(authenticated),
        canWrite: ref(canWrite),
        canManage: ref(canManage)
    }
    vi.mocked(useSession).mockReturnValue({
        ...state,
        enabled: computed(() => true),
        role: computed(() => (canManage ? 'admin' : canWrite ? 'guest' : 'none')),
        displayName: computed(() => ''),
        signIn: vi.fn(),
        signOut: vi.fn()
    } as unknown as ReturnType<typeof useSession>)
}

describe('useWriteGuard', () => {
    it('arms a manage control (the default scope) for an administrator', () => {
        session(true, true, true)

        expect(useWriteGuard().disabled.value).toBe(false)
    })

    it('disables a manage control for a guest, who holds canWrite but not canManage', () => {
        session(true, true, false)

        expect(useWriteGuard('manage').disabled.value).toBe(true)
    })

    it('arms a data control for a guest', () => {
        session(true, true, false)

        expect(useWriteGuard('data').disabled.value).toBe(false)
    })

    it('disables it for a visitor, and says why', () => {
        session(false, false, false)

        const { disabled, reason } = useWriteGuard()

        expect(disabled.value).toBe(true)
        expect(reason.value).toBe('Connexion requise pour modifier les données')
    })

    it('disables it for a signed-in account without the rights, and says why', () => {
        session(true, false, false)

        const { disabled, reason } = useWriteGuard()

        expect(disabled.value).toBe(true)
        expect(reason.value).toBe("Votre compte n'a pas les droits d'édition")
    })
})
