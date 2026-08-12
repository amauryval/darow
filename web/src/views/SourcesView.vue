<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage } from '../client/http'
import { useSources } from '../composables/useSources'
import { useToast } from '../composables/useToast'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import { useWriteGuard } from '../composables/useWriteGuard'
import { parseSorting, stringifySorting } from '../utils/sort'
import { queryString } from '../utils/routeQuery'
import { RefreshCw, UploadCloud } from '@lucide/vue'
import Badge from '../components/ui/Badge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import Typography from '../components/ui/Typography.vue'
import SourcesTable from '../components/sources/SourcesTable.vue'
import SourceUploadModal from '../components/sources/SourceUploadModal.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

useDocumentTitle(computed(() => 'Sources'))

const sourcesState = useSources(
    queryString(route.query.sort) ? parseSorting(queryString(route.query.sort)) : undefined
)
const { sources, total, loading, loadedOnce, sorting, refresh, uploading, upload } = sourcesState
const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

// The URL carries the screen state (§9): a refresh or a shared link keeps
// the chosen sort.
watch(sorting, (sort) => {
    router.replace({ query: { ...route.query, sort: stringifySorting(sort) || undefined } })
})

const uploadOpen = ref(false)

// Counted on the loaded page only — the list is paginated, so this describes
// what is on screen, not the whole workDir.
const loadedCount = computed(() => sources.value.filter((source) => source.loaded).length)

async function handleUpload(file: File) {
    try {
        const result = await upload(file)
        uploadOpen.value = false
        if (result) {
            toast.success(`${file.name} chargé.`)
            router.push(`/tables/${result.tableId}`)
        } else {
            toast.success(`${file.name} ajouté — choisissez une couche à charger.`)
        }
    } catch (e) {
        toast.error(errorMessage(e, `Impossible de charger ${file.name}`))
    }
}
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
        <Breadcrumbs :items="[{ label: 'Sources' }]" />

        <div class="flex shrink-0 flex-wrap items-center justify-between gap-3">
            <div class="flex flex-col gap-1">
                <Typography variant="body-sm"
                    >Fichiers détectés dans le répertoire de travail du serveur.</Typography
                >
                <div v-if="loadedOnce" class="flex items-center gap-2">
                    <Badge tone="neutral">{{ total }} fichier{{ total > 1 ? 's' : '' }}</Badge>
                    <Badge v-if="loadedCount > 0" tone="success"
                        >{{ loadedCount }} chargé{{ loadedCount > 1 ? 's' : '' }}</Badge
                    >
                </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <BaseButton
                    variant="secondary"
                    size="sm"
                    :icon="RefreshCw"
                    :disabled="loading"
                    @click="refresh"
                    >Rafraîchir</BaseButton
                >
                <BaseButton
                    variant="primary"
                    size="sm"
                    :icon="UploadCloud"
                    :busy="uploading"
                    :disabled="uploading || writeDisabled"
                    :title="writeDisabled ? writeGuardReason : undefined"
                    @click="uploadOpen = true"
                >
                    Importer un fichier
                </BaseButton>
            </div>
        </div>

        <SourcesTable :state="sourcesState" />

        <SourceUploadModal
            v-if="uploadOpen"
            :uploading="uploading"
            @close="uploadOpen = false"
            @upload="handleUpload"
        />
    </div>
</template>
