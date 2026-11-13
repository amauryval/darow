<script setup lang="ts">
import { watch } from 'vue'
import { useCanEdit } from '@/darow/composables/useCanEdit'
import { useColumns } from '@/darow/composables/useColumns'
import { useSettings } from '@/darow/composables/useSettings'
import { useTable } from '@/darow/composables/useTable'
import ColumnsModal from './ColumnsModal.vue'
import RowModal from './RowModal.vue'
import TableSettingsModal from './TableSettingsModal.vue'

const columnsOpen = defineModel<boolean>('columns', { required: true })
const settingsOpen = defineModel<boolean>('settings', { required: true })

const { canEdit } = useCanEdit()
const table = useTable()
const { name, row, conditions, attributes, hasGeometry, detail, tilesUrl } = table
const columns = useColumns(name, table.changed)
const { table: tableSettings, busy: settingsBusy, loadTable, saveTable } = useSettings()

watch(settingsOpen, async (open) => {
    if (open && !(await loadTable(name.value))) settingsOpen.value = false
})

watch(columnsOpen, (open) => {
    if (!open) columns.clearPreview()
})

const onSaveSettings = async (values: Parameters<typeof saveTable>[1]) => {
    if (await saveTable(name.value, values)) {
        settingsOpen.value = false
        await table.reload()
    }
}
</script>

<template>
    <RowModal
        :row="row"
        :columns="attributes"
        :has-geometry="hasGeometry"
        :can-edit="canEdit"
        :tiles-url="tilesUrl"
        :extent="detail?.extent"
        @save="table.saveRow"
        @remove="table.deleteRow"
        @close="table.selectRow(null)"
    />
    <ColumnsModal
        :is-open="columnsOpen"
        :columns="attributes"
        :preview="columns.preview.value"
        :filtered="conditions !== ''"
        :busy="columns.busy.value"
        @add="columns.add"
        @rename="columns.rename"
        @drop="columns.drop"
        @preview="(column, expression) => columns.previewTransform(column, expression, conditions)"
        @transform="(column, expression) => columns.transform(column, expression, conditions)"
        @close="columnsOpen = false"
    />
    <TableSettingsModal
        :settings="settingsOpen ? tableSettings : null"
        :busy="settingsBusy"
        @save="onSaveSettings"
        @close="settingsOpen = false"
    />
</template>
