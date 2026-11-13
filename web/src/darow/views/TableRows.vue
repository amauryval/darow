<script setup lang="ts">
import { computed, useTemplateRef } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import EmptyState from '@myapplibs/web-platform/components/base/EmptyState.vue'
import DataTable from '@myapplibs/web-platform/components/table/DataTable.vue'
import EditableCell from '@myapplibs/web-platform/components/table/EditableCell.vue'
import type { DataColumn, DataTableHandle } from '@myapplibs/web-platform/components/table/types'
import type { Column, Rows } from '@/darow/api/engine.gen'
import { ID_COLUMN } from '@/darow/utils/conditions'
import { COLUMN_PREFIX, rowIdOf, rowSortOf, textOf, type RowSort } from '@/darow/utils/rowValues'

interface Line {
    id: string
    values: Record<string, unknown>
}

const props = defineProps<{ table: string; columns: Column[]; rows: Rows; canEdit: boolean }>()
const emit = defineEmits<{
    select: [row: Record<string, unknown>]
    sort: [sort: RowSort]
    edit: [id: number, column: string, value: string]
}>()
const selected = defineModel<number[]>('selected', { required: true })

const grid = useTemplateRef<DataTableHandle>('grid')

defineExpose({ picker: computed(() => grid.value?.picker) })

const lines = computed<Line[]>(() =>
    props.rows.rows.map((values) => ({ id: String(rowIdOf(values)), values }))
)

const selectedLines = computed({
    get: () => selected.value.map(String),
    set: (ids: string[]) => (selected.value = ids.map(Number))
})

const dataColumns = computed<DataColumn<Line>[]>(() => [
    {
        id: ID_COLUMN,
        label: t('darow.table.id'),
        size: 80,
        sortValue: (line: Line) => rowIdOf(line.values) ?? undefined
    },
    ...props.columns.map((column) => ({
        id: `${COLUMN_PREFIX}${column.name}`,
        label: column.name,
        size: 160,
        sortValue: (line: Line) => textOf(line.values[column.name]),
        cell: (line: Line) => textOf(line.values[column.name])
    }))
])
</script>

<template>
    <div class="flex flex-col gap-2 min-h-0">
        <EmptyState v-if="!lines.length" :message="t('darow.table.noRow')" />
        <template v-else>
            <div class="flex-1 min-h-0 overflow-auto rounded-lg border border-base-300">
                <DataTable
                    ref="grid"
                    :key="table"
                    :rows="lines"
                    v-model:selected="selectedLines"
                    :columns="dataColumns"
                    selectable
                    server-sorting
                    :sizing-key="`darow-${table}-sizing`"
                    :sorting-key="`darow-${table}-sorting`"
                    :visibility-key="`darow-${table}-columns`"
                    @sort="emit('sort', rowSortOf($event))"
                >
                    <template #cell-__id="{ row }">
                        <BaseButton
                            tone="ghost"
                            size="xs"
                            :title="t('darow.row.open')"
                            @click="emit('select', row.values)"
                        >
                            {{ row.id }}
                        </BaseButton>
                    </template>
                    <template
                        v-for="column in columns"
                        :key="column.name"
                        #[`cell-${COLUMN_PREFIX}${column.name}`]="{ row }"
                    >
                        <EditableCell
                            :value="textOf(row.values[column.name])"
                            :label="column.name"
                            :disabled="!canEdit"
                            @save="emit('edit', Number(row.id), column.name, $event)"
                        />
                    </template>
                </DataTable>
            </div>
        </template>
    </div>
</template>
