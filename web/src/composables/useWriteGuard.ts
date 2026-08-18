import { computed } from 'vue'
import { useSession } from './useSession'

// A thin read of useSession's verdict for a write control, so a view or a
// domain component disables its button without threading the whole session
// through. Never consumed by a components/ui/ primitive (rule 0.5):
// BaseButton/ConfirmButton keep taking a plain disabled/title prop, and stay
// domain-free — the caller passes this composable's output down as those
// props.
export function useWriteGuard() {
    const { authenticated, canWrite } = useSession()

    const disabled = computed(() => !authenticated.value || !canWrite.value)

    const reason = computed(() =>
        !authenticated.value
            ? 'Connexion requise pour modifier les données'
            : "Votre compte n'a pas les droits d'édition"
    )

    return { disabled, reason }
}
