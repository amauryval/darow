import { ref } from 'vue'
import { errorMessage } from '../client/http'

// The async-write shape written out at every mutation call site: a loading
// flag raised for the attempt, an error cleared before it and set from
// whatever failed, and the action's own result — or `null`, the signal every
// caller branches on (§8.4) to choose between an inline message and a toast.
// The mapping to French text stays errorMessage's, never assembled here.
export function useAsyncAction() {
    const isLoading = ref(false)
    const error = ref('')

    async function run<T>(action: () => Promise<T>, fallbackMessage: string): Promise<T | null> {
        isLoading.value = true
        error.value = ''
        try {
            return await action()
        } catch (e) {
            error.value = errorMessage(e, fallbackMessage)
            return null
        } finally {
            isLoading.value = false
        }
    }

    return { isLoading, error, run }
}
