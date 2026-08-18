import { computed } from 'vue'
import { useAuth } from './useAuth'

// The single verdict every screen reads. Rebuilt on useAuth
// rather than reading the raw session itself, so the mapping from the
// module's roles to one display label lives in exactly one place.
export function useSession() {
    const {
        enabled,
        loggedIn,
        authorized,
        isAdmin,
        isGuest,
        displayName,
        signIn,
        signOut,
        refreshSession
    } = useAuth()

    void refreshSession()

    const authenticated = computed(() => loggedIn.value)

    // One label, never a list: a signed-in user holding neither application
    // role sits exactly where an anonymous visitor sits (back §6.7), so
    // 'none' reads the same as "not signed in" everywhere the label is
    // shown — only the account menu tells the two apart.
    const role = computed<'admin' | 'guest' | 'none'>(() => {
        if (isAdmin.value) return 'admin'
        if (isGuest.value) return 'guest'
        return 'none'
    })

    const canWrite = computed(() => authorized.value && role.value === 'admin')

    return {
        enabled,
        authenticated,
        role,
        canWrite,
        displayName,
        signIn,
        signOut
    }
}
