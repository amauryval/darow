import { computed } from 'vue'
import { useSession } from './useSession'

// A thin read of useSession's verdict for a write control, so a view or a
// domain component disables its button without threading the whole session
// through. Never consumed by a components/ui/ primitive (rule 0.5):
// BaseButton/ConfirmButton keep taking a plain disabled/title prop, and stay
// domain-free — the caller passes this composable's output down as those
// props.
//
// scope picks which of useSession's two privileges the button needs:
// 'data' (rows, columns, filters — a guest holds it too) or 'manage'
// (everything structural, source loading included — admin alone). Defaults
// to 'manage', the stricter of the two, so a call site left unchanged keeps
// its prior behaviour.
export function useWriteGuard(scope: 'data' | 'manage' = 'manage') {
    const { authenticated, canWrite, canManage } = useSession()

    const permitted = computed(() => (scope === 'data' ? canWrite.value : canManage.value))
    const disabled = computed(() => !authenticated.value || !permitted.value)

    const reason = computed(() =>
        !authenticated.value
            ? 'Connexion requise pour modifier les données'
            : "Votre compte n'a pas les droits d'édition"
    )

    return { disabled, reason }
}
