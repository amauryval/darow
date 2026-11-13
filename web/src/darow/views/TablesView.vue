<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '@myapplibs/web-platform/i18n'
import ListDetailLayout from '@myapplibs/web-platform/components/layout/ListDetailLayout.vue'
import ListRow from '@myapplibs/web-platform/components/layout/ListRow.vue'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Loading from '@myapplibs/web-platform/components/base/Loading.vue'
import PageTemplate from '@myapplibs/web-platform/components/layout/PageTemplate.vue'
import ColumnPicker from '@myapplibs/web-platform/components/table/ColumnPicker.vue'
import TablePager from '@myapplibs/web-platform/components/table/TablePager.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { ExportFormat } from '@/darow/api/darow'
import { useCanEdit } from '@/darow/composables/useCanEdit'
import { usePanes } from '@/darow/composables/usePanes'
import { useSavedFilters } from '@/darow/composables/useSavedFilters'
import { useTable } from '@/darow/composables/useTable'
import { useTables } from '@/darow/composables/useTables'
import { icons } from '@/darow/icons'
import { DarowRoutes } from '@/darow/routeNames'
import ExportModal from './ExportModal.vue'
import FiltersBar from './FiltersBar.vue'
import FiltersModal from './FiltersModal.vue'
import NameModal from './NameModal.vue'
import PanesBar from './PanesBar.vue'
import TableEditors from './TableEditors.vue'
import TableHeader from './TableHeader.vue'
import TableMap from './TableMap.vue'
import TableRows from './TableRows.vue'

const route = useRoute()
const router = useRouter()
const { canEdit, canAdmin } = useCanEdit()
const { search, items, load } = useTables()
const table = useTable()
const {
    name,
    detail,
    rows,
    page,
    pageSize,
    draft,
    applied,
    loading,
    attributes,
    hasGeometry,
    pending,
    selected,
    selectionOnly,
    tilesUrl,
    links
} = table
const saved = useSavedFilters()
const tableFilters = saved.forTable(name)

const panes = usePanes(hasGeometry)
const grid = useTemplateRef<InstanceType<typeof TableRows>>('grid')
const renaming = ref(false)
const filtering = ref(false)
const editingColumns = ref(false)
const editingSettings = ref(false)
const exporting = ref(false)

const routeName = computed(() => (typeof route.params.name === 'string' ? route.params.name : ''))
const routeFilter = computed(() =>
    typeof route.query.filter === 'string' ? route.query.filter : ''
)

const select = (next: string) =>
    router.replace({ name: DarowRoutes.TABLES, params: next ? { name: next } : {} })

onMounted(async () => {
    await Promise.all([load(), saved.load()])
    await table.open(routeName.value)
    const filter = saved.get(routeFilter.value)
    if (filter && tableFilters.value.includes(filter)) await table.toggleFilter(filter)
})

watch(routeName, (next) => {
    if (next !== name.value) void table.open(next)
})

const onRename = async (to: string) => {
    renaming.value = false
    if (await table.rename(to)) await select(to)
}

const onRemove = async () => {
    if (await table.remove()) await select('')
}

const onSaveFilter = (filterName: string) =>
    saved.save('', { table: name.value, name: filterName, where: draft.value })

const onManage = () => router.push({ name: DarowRoutes.FILTERS })

const onApply = (where?: string) => {
    filtering.value = false
    return table.apply(where)
}

const onExport = (format: ExportFormat, srid: number) => {
    exporting.value = false
    table.download(format, srid)
}

const breadcrumbs = computed(() => [{ label: t('darow.pages.tables'), icon: icons.Tables }])
</script>

<template>
    <PageTemplate fill-height :breadcrumbs="breadcrumbs">
        <ListDetailLayout
            v-model:query="search"
            :has-selection="!!detail"
            :search-placeholder="t('darow.tables.search')"
            :empty-detail-message="t('darow.tables.select')"
            @close="select('')"
        >
            <template #list>
                <ListRow
                    v-for="item in items"
                    :key="item.name"
                    :label="item.name"
                    :selected="item.name === name"
                    @select="select(item.name)"
                />
                <p v-if="!items.length" class="px-3 py-2 text-sm text-base-content/70">
                    {{ t('darow.tables.empty') }}
                </p>
            </template>

            <template #detail-header>
                <TableHeader
                    v-if="detail"
                    :name="name"
                    :can-edit="canEdit"
                    :can-admin="canAdmin"
                    :links="links"
                    @new-row="table.newRow"
                    @columns="editingColumns = true"
                    @settings="editingSettings = true"
                    @rename="renaming = true"
                    @export="exporting = true"
                />
            </template>

            <template #detail>
                <div v-if="detail" class="flex flex-col gap-4 h-full min-h-0">
                    <div class="flex items-start gap-2 shrink-0">
                        <FiltersBar
                            class="flex-1 min-w-0"
                            :applied="applied"
                            :selected="selected.length"
                            :selection-only="selectionOnly"
                            :filters="tableFilters"
                            :active-id="table.activeId.value"
                            @open="filtering = true"
                            @unapply="table.unapply"
                            @toggle="table.toggleFilter"
                            @toggle-selection="table.toggleSelection"
                            @clear-selection="table.clearSelection"
                        />
                        <PanesBar
                            v-if="hasGeometry || grid?.picker"
                            :has-geometry="hasGeometry"
                            :show-map="panes.showMap.value"
                            :show-table="panes.showTable.value"
                            @toggle-map="panes.toggleMap"
                            @toggle-table="panes.toggleTable"
                        >
                            <ColumnPicker
                                v-if="grid?.picker"
                                :columns="grid.picker.columns"
                                :shown="grid.picker.shown"
                                @toggle="grid.picker.toggle"
                                @reset="grid.picker.reset"
                            />
                        </PanesBar>
                    </div>
                    <Loading v-if="loading" class="shrink-0" />
                    <div
                        class="grid gap-4 flex-1 min-h-0"
                        :class="
                            panes.split.value
                                ? 'grid-rows-[minmax(16rem,1fr)_minmax(0,1fr)] xl:grid-rows-1 xl:grid-cols-2'
                                : 'grid-rows-1'
                        "
                    >
                        <TableMap
                            v-if="panes.showMap.value"
                            class="min-h-64 min-w-0"
                            :tiles-url="tilesUrl"
                            :extent="detail.extent"
                            :selected="selected"
                            :can-edit="canEdit"
                            @select="table.openRow"
                            @pick="table.pick"
                            @clear="table.clearSelection"
                        />
                        <TableRows
                            v-if="panes.showTable.value"
                            ref="grid"
                            class="min-h-0 min-w-0"
                            :table="name"
                            :columns="attributes"
                            :selected="selected"
                            @update:selected="table.setSelected"
                            :rows="rows"
                            :can-edit="canEdit"
                            @sort="table.setSort"
                            @edit="table.saveCell"
                            @select="table.selectRow"
                        />
                    </div>
                    <div
                        class="flex flex-wrap items-center justify-between gap-2 shrink-0 pt-3 border-t border-base-content/10"
                    >
                        <BaseButton
                            v-if="canAdmin"
                            variant="danger"
                            tone="soft"
                            size="sm"
                            @click="onRemove"
                        >
                            <Icon :icon-name="icons.Delete" />
                            {{ t('darow.table.delete') }}
                        </BaseButton>
                        <TablePager
                            class="ml-auto"
                            :page="page"
                            :count="rows.total"
                            :page-size="pageSize"
                            @update:page="table.setPage"
                        />
                    </div>
                </div>
            </template>
        </ListDetailLayout>

        <FiltersModal
            v-if="detail"
            v-model:where="draft"
            :is-open="filtering"
            :columns="attributes"
            :pending="pending"
            :can-edit="canEdit"
            @apply="onApply()"
            @save="onSaveFilter"
            @manage="onManage"
            @close="filtering = false"
        />
        <TableEditors v-model:columns="editingColumns" v-model:settings="editingSettings" />
        <ExportModal
            :is-open="exporting"
            :has-geometry="hasGeometry"
            @export="onExport"
            @close="exporting = false"
        />
        <NameModal
            :is-open="renaming"
            :title="t('darow.table.rename')"
            :label="t('darow.table.name')"
            :initial="name"
            @submit="onRename"
            @close="renaming = false"
        />
    </PageTemplate>
</template>
