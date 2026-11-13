import { computed } from 'vue'
import { useAuth } from '@myapplibs/web-platform/components/composables/useAuth'

export function useCanEdit() {
    const { authEnabled, isAdmin, isGuest } = useAuth()
    const open = computed(() => authEnabled.value === false)
    return {
        canEdit: computed(() => open.value || isAdmin.value || isGuest.value),
        canAdmin: computed(() => open.value || isAdmin.value)
    }
}
