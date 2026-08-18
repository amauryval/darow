import { describe, expect, it, vi } from 'vitest'
import { computed, ref } from 'vue'
import { useWriteGuard } from './useWriteGuard'
import { useSession } from './useSession'

vi.mock('./useSession', () => ({ useSession: vi.fn() }))

function session(authenticated: boolean, canWrite: boolean) {
    const state = { authenticated: ref(authenticated), canWrite: ref(canWrite) }
    vi.mocked(useSession).mockReturnValue({
        ...state,
        enabled: computed(() => true),
        role: computed(() => (canWrite ? 'admin' : 'guest')),
        displayName: computed(() => ''),
        signIn: vi.fn(),
        signOut: vi.fn()
    } as unknown as ReturnType<typeof useSession>)
}

describe('useWriteGuard', () => {
    it('arms a write control for a signed-in account holding the rights', () => {
        session(true, true)

        expect(useWriteGuard().disabled.value).toBe(false)
    })

    it('disables it for a visitor, and says why', () => {
        session(false, false)

        const { disabled, reason } = useWriteGuard()

        expect(disabled.value).toBe(true)
        expect(reason.value).toBe('Connexion requise pour modifier les données')
    })

    it('disables it for a signed-in account without the rights, and says why', () => {
        session(true, false)

        const { disabled, reason } = useWriteGuard()

        expect(disabled.value).toBe(true)
        expect(reason.value).toBe("Votre compte n'a pas les droits d'édition")
    })
})
