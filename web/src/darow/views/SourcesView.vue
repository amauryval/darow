<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { formatDate, formatSize, t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Card from '@myapplibs/web-platform/components/base/Card.vue'
import EmptyState from '@myapplibs/web-platform/components/base/EmptyState.vue'
import FormFileInput from '@myapplibs/web-platform/components/form/FormFileInput.vue'
import PageTemplate from '@myapplibs/web-platform/components/layout/PageTemplate.vue'
import DataTable from '@myapplibs/web-platform/components/table/DataTable.vue'
import type { DataColumn } from '@myapplibs/web-platform/components/table/types'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { Source } from '@/darow/api/sources.gen'
import { useCanEdit } from '@/darow/composables/useCanEdit'
import { useSources } from '@/darow/composables/useSources'
import { SOURCE_FORMATS } from '@/darow/utils/catalog'
import { icons } from '@/darow/icons'
import LoadSourceModal from './LoadSourceModal.vue'

interface Line extends Source {
    id: string
}

const { canAdmin } = useCanEdit()
const { sources, progress, load, send, cancel, remove, layers, loadInto } = useSources()

const file = ref<File | null>(null)
const loading = ref<Source | null>(null)
const loadingLayers = shallowRef<string[]>([])

onMounted(load)

const lines = computed<Line[]>(() =>
    sources.value.map((source) => ({ ...source, id: source.name }))
)

const columns = computed<DataColumn<Line>[]>(() => [
    {
        id: 'name',
        label: t('darow.sources.name'),
        size: 280,
        sortValue: (s) => s.name,
        cell: (s) => s.name
    },
    {
        id: 'size',
        label: t('darow.sources.size'),
        size: 120,
        sortValue: (s) => s.size,
        cell: (s) => formatSize(s.size)
    },
    {
        id: 'modifiedAt',
        label: t('darow.sources.modifiedAt'),
        size: 180,
        sortValue: (s) => s.modifiedAt,
        cell: (s) => formatDate(s.modifiedAt)
    },
    { id: 'actions', label: '', size: 100, resizable: false }
])

const onSend = async () => {
    if (!file.value) return
    if (await send(file.value)) file.value = null
}

const openLoad = async (source: Source) => {
    loadingLayers.value = await layers(source.name)
    loading.value = source
}

const onLoad = async (request: { table: string; layer: string; srid: number }) => {
    const source = loading.value
    loading.value = null
    if (source) await loadInto(source.name, request)
}

const breadcrumbs = computed(() => [{ label: t('darow.pages.sources'), icon: icons.Sources }])
</script>

<template>
    <PageTemplate :breadcrumbs="breadcrumbs">
        <div class="flex flex-col gap-4">
            <Card v-if="canAdmin" :title="t('darow.sources.upload')">
                <div class="flex flex-wrap items-end gap-2">
                    <FormFileInput
                        v-model="file"
                        :legend="t('darow.sources.file')"
                        :accepted-formats="SOURCE_FORMATS"
                        :progress="progress"
                        @cancel="cancel"
                    />
                    <BaseButton
                        variant="primary"
                        size="sm"
                        :disabled="!file || progress !== null"
                        @click="onSend"
                    >
                        <Icon :icon-name="icons.Load" />
                        {{ t('darow.sources.send') }}
                    </BaseButton>
                </div>
            </Card>
            <EmptyState v-if="!lines.length" :message="t('darow.sources.empty')" />
            <DataTable
                v-else
                :rows="lines"
                :columns="columns"
                sizing-key="darow-sources-sizing"
                sorting-key="darow-sources-sorting"
            >
                <template #cell-actions="{ row }">
                    <div v-if="canAdmin" class="flex gap-1">
                        <BaseButton
                            tone="ghost"
                            size="xs"
                            :title="t('darow.sources.load')"
                            :aria-label="t('darow.sources.load')"
                            @click="openLoad(row)"
                        >
                            <Icon :icon-name="icons.Tables" />
                        </BaseButton>
                        <BaseButton
                            tone="ghost"
                            size="xs"
                            :title="t('darow.sources.delete')"
                            :aria-label="t('darow.sources.delete')"
                            @click="remove(row.name)"
                        >
                            <Icon :icon-name="icons.Delete" />
                        </BaseButton>
                    </div>
                </template>
            </DataTable>
        </div>
        <LoadSourceModal
            :source="loading?.name ?? ''"
            :layers="loadingLayers"
            @load="onLoad"
            @close="loading = null"
        />
    </PageTemplate>
</template>
