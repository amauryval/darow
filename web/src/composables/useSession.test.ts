import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useSession } from './useSession'
import { useAuth } from './useAuth'

vi.mock('./useAuth', () => ({ useAuth: vi.fn() }))

function auth(state: {
    loggedIn?: boolean
    authorized?: boolean
    admin?: boolean
    guest?: boolean
}) {
    const refreshSession = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useAuth).mockReturnValue({
        enabled: ref(true),
        loggedIn: ref(state.loggedIn ?? false),
        authorized: ref(state.authorized ?? false),
        isAdmin: ref(state.admin ?? false),
        isGuest: ref(state.guest ?? false),
        displayName: ref('Amaury'),
        signIn: vi.fn(),
        signOut: vi.fn(),
        refreshSession
    } as unknown as ReturnType<typeof useAuth>)
    return refreshSession
}

describe('useSession', () => {
    it('refreshes the session as soon as a screen reads it', () => {
        const refreshSession = auth({})

        useSession()

        expect(refreshSession).toHaveBeenCalled()
    })

    it('reads an administrator as holding both write privileges', () => {
        auth({ loggedIn: true, authorized: true, admin: true })

        const { authenticated, role, canWrite, canManage } = useSession()

        expect(authenticated.value).toBe(true)
        expect(role.value).toBe('admin')
        expect(canWrite.value).toBe(true)
        expect(canManage.value).toBe(true)
    })

    it('reads a guest as signed in, allowed to write data but not to manage', () => {
        auth({ loggedIn: true, authorized: true, guest: true })

        const { role, canWrite, canManage } = useSession()

        expect(role.value).toBe('guest')
        expect(canWrite.value).toBe(true)
        expect(canManage.value).toBe(false)
    })

    it('puts a signed-in account holding no application role where a visitor sits', () => {
        auth({ loggedIn: true, authorized: false })

        const { authenticated, role, canWrite, canManage } = useSession()

        expect(authenticated.value).toBe(true)
        expect(role.value).toBe('none')
        expect(canWrite.value).toBe(false)
        expect(canManage.value).toBe(false)
    })

    it('refuses both privileges to an unauthorized administrator', () => {
        auth({ loggedIn: true, authorized: false, admin: true })

        expect(useSession().canWrite.value).toBe(false)
        expect(useSession().canManage.value).toBe(false)
    })
})
