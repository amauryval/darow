<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage } from '../client/http'
import { useSources } from '../composables/useSources'
import { useSourceLoad } from '../composables/useSourceLoad'
import { useTableProvenance } from '../composables/useTableProvenance'
import { useToast } from '../composables/useToast'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import { useWriteGuard } from '../composables/useWriteGuard'
import { parseSorting, stringifySorting } from '../utils/sort'
import { queryString } from '../utils/routeQuery'
import { RefreshCw, UploadCloud } from '@lucide/vue'
import { layerKey } from '../composables/useSources'
import { suggestTableName } from '../utils/tableNames'
import type { SourceRow } from '../utils/sourceRows'
import Badge from '../components/ui/Badge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import Typography from '../components/ui/Typography.vue'
import SourcesTable from '../components/sources/SourcesTable.vue'
import SourceLoadModal from '../components/sources/SourceLoadModal.vue'
import SourceUploadModal from '../components/sources/SourceUploadModal.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

useDocumentTitle(computed(() => 'Sources'))

const sourcesState = useSources(
    queryString(route.query.sort) ? parseSorting(queryString(route.query.sort)) : undefined
)
const { total, loading, loadedOnce, sorting, refresh, uploading, upload } = sourcesState
const provenance = useTableProvenance()
const load = useSourceLoad()

onMounted(provenance.reload)
const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

// The URL carries the screen state (§9): a refresh or a shared link keeps
// the chosen sort.
watch(sorting, (sort) => {
    router.replace({ query: { ...route.query, sort: stringifySorting(sort) || undefined } })
})

const uploadOpen = ref(false)

// The name and the tables of whatever the load form is open on: a layer row
// is named after its layer, a file row after the file (§3.2 back).
const loadLabel = computed(() => {
    const target = load.loadTarget.value
    if (!target) return ''
    return target.layerName ? layerKey(target.source, target.layerName) : target.source.name
})

const loadExisting = computed(() => {
    const target = load.loadTarget.value
    return target ? provenance.tablesOf(target.source.name, target.layerName) : []
})

const suggestedName = computed(() =>
    loadLabel.value ? suggestTableName(loadLabel.value, provenance.tableNames()) : ''
)

async function onConfirmLoad(options: Parameters<typeof load.confirmLoad>[0]) {
    if (await load.confirmLoad(options)) await provenance.reload()
}

async function handleUpload(file: File) {
    try {
        await upload(file)
        uploadOpen.value = false
        toast.success(`${file.name} importé — chargez-le pour en faire une table.`)
    } catch (e) {
        toast.error(errorMessage(e, `Impossible d'importer ${file.name}`))
    }
}
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
        <Breadcrumbs :items="[{ label: 'Sources' }]" />

        <div class="flex shrink-0 flex-wrap items-center justify-between gap-3">
            <div class="flex flex-col gap-1">
                <Typography variant="body-sm"
                    >Fichiers détectés dans le répertoire de travail du serveur. Charger un fichier
                    en fait une table indépendante : supprimer le fichier ensuite ne la touche
                    pas.</Typography
                >
                <div v-if="loadedOnce" class="flex items-center gap-2">
                    <Badge tone="neutral">{{ total }} fichier{{ total > 1 ? 's' : '' }}</Badge>
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

        <SourcesTable
            :state="sourcesState"
            :provenance="provenance"
            @start-load="(row: SourceRow) => load.startLoad(row.source, row.layerName)"
        />

        <SourceLoadModal
            v-if="load.loadTarget.value"
            :source-label="loadLabel"
            :suggested-name="suggestedName"
            :existing="loadExisting"
            :detected-srid="load.detectedSrid.value"
            :detecting="load.epsgDetecting.value"
            :epsg-applies="load.epsgApplies.value"
            :loading="load.loading.value"
            @close="load.cancelLoad"
            @confirm="onConfirmLoad"
        />

        <SourceUploadModal
            v-if="uploadOpen"
            :uploading="uploading"
            @close="uploadOpen = false"
            @upload="handleUpload"
        />
    </div>
</template>
