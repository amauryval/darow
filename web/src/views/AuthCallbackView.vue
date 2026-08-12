<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
    clearSignedIn,
    forgetSilentAttempt,
    markSignedIn,
    oidcManager,
    silentAttemptReturnTo
} from '../auth/oidc'
import Typography from '../components/ui/Typography.vue'

// The code exchange usually settles in well under a second — this is only
// the ceiling past which the page admits it may be stuck rather than
// staying a wordless blank screen.
const CALLBACK_TIMEOUT_MS = 20000
const STALLED_AFTER_MS = CALLBACK_TIMEOUT_MS / 4

// prompt=none (the boot recovery's silent signin) answers one of these when
// nobody is in fact signed in at the provider — an ordinary "not signed in"
// outcome, not an error: the flag is cleared and the visit returns to the
// page it was headed for.
const SILENT_FAILURES = ['login_required', 'interaction_required', 'consent_required']

const router = useRouter()
const error = ref<string | null>(null)
const stalled = ref(false)

function timeout(ms: number): Promise<never> {
    return new Promise((_, reject) =>
        setTimeout(
            () => reject(new Error(`Le fournisseur d'identité n'a pas répondu en ${ms / 1000}s.`)),
            ms
        )
    )
}

onMounted(async () => {
    const watchdog = setTimeout(() => (stalled.value = true), STALLED_AFTER_MS)
    try {
        const user = await Promise.race([
            oidcManager().signinRedirectCallback(),
            timeout(CALLBACK_TIMEOUT_MS)
        ])
        forgetSilentAttempt()
        markSignedIn()
        const returnTo = typeof user.state === 'string' ? user.state : null
        await router.replace(returnTo ?? '/dashboard')
    } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        if (SILENT_FAILURES.some((failure) => message.includes(failure))) {
            clearSignedIn()
            const returnTo = silentAttemptReturnTo()
            forgetSilentAttempt()
            await router.replace(returnTo ?? '/dashboard')
            return
        }
        error.value = message
    } finally {
        clearTimeout(watchdog)
    }
})
</script>

<template>
    <div class="flex min-h-screen flex-col items-center justify-center gap-4 p-8 text-center">
        <template v-if="!error">
            <Typography variant="body">Connexion en cours…</Typography>
            <Typography v-if="stalled" variant="caption">
                L'échange avec le fournisseur d'identité prend plus de temps que prévu.
            </Typography>
        </template>
        <template v-else>
            <Typography variant="body">La connexion a échoué : {{ error }}</Typography>
            <RouterLink to="/dashboard" class="link">Retour au tableau de bord</RouterLink>
        </template>
    </div>
</template>
