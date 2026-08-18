<script setup lang="ts">
import { ChevronRight } from '@lucide/vue'
import { useSqlSchemaReference } from '../../composables/useSqlSchemaReference'
import ColumnPickerList from './ColumnPickerList.vue'
import ListFooter from '../ui/ListFooter.vue'
import Spinner from '../ui/Spinner.vue'
import Typography from '../ui/Typography.vue'

// A reference next to the free-SQL textarea (DeriveSqlForm): the registered
// tables and, expanded, their columns — a click inserts the identifier at
// the caret instead of the user copying it from another tab. A table
// inserts its physicalName, not tableName: the query runs straight against
// DuckDB (back service/derive.go), which never heard of the friendly name.
const emit = defineEmits<{ insert: [text: string] }>()

// Kept as one object rather than destructured: `sentinel` is a Ref that
// ListFooter expects as-is, and a top-level destructured binding would be
// auto-unwrapped by the template compiler like every other ref here.
const list = useSqlSchemaReference()
</script>

<template>
    <div class="flex h-full flex-col">
        <Typography variant="eyebrow" class="mb-2 shrink-0">Tables</Typography>
        <div class="min-h-0 flex-1 overflow-y-auto rounded border border-base-300">
            <p v-if="list.error.value" class="p-3 text-xs text-error" role="alert">
                {{ list.error.value }}
            </p>
            <p
                v-else-if="!list.loadedOnce.value && list.loading.value"
                class="p-3 text-xs text-base-content/40"
            >
                Chargement…
            </p>
            <p
                v-else-if="list.loadedOnce.value && list.tables.value.length === 0"
                class="p-3 text-xs text-base-content/40"
            >
                Aucune table.
            </p>
            <template v-else>
                <div
                    v-for="table in list.tables.value"
                    :key="table.tableId"
                    class="border-b border-base-300 last:border-0"
                >
                    <div class="flex items-center gap-1 pr-2 hover:bg-base-200">
                        <button
                            type="button"
                            class="p-1.5"
                            :aria-label="
                                list.expanded.value === table.tableId ? 'Réduire' : 'Déplier'
                            "
                            @click="list.toggle(table.tableId)"
                        >
                            <ChevronRight
                                class="size-3 shrink-0 transition-transform"
                                :class="{ 'rotate-90': list.expanded.value === table.tableId }"
                            />
                        </button>
                        <button
                            type="button"
                            class="flex-1 truncate py-1.5 text-left text-xs"
                            :title="`Insérer ${table.physicalName}`"
                            @click="emit('insert', table.physicalName)"
                        >
                            {{ table.tableName }}
                        </button>
                    </div>
                    <div v-if="list.expanded.value === table.tableId" class="pb-1 pl-6">
                        <Spinner v-if="list.loadingColumns.value === table.tableId" />
                        <ColumnPickerList
                            :columns="list.columnsByTable.value[table.tableId] ?? []"
                            @insert="emit('insert', $event)"
                        />
                    </div>
                </div>
                <ListFooter
                    :sentinel="list.sentinel"
                    :loading="list.loadedOnce.value && list.loading.value"
                    :loaded="list.tables.value.length"
                    :total="list.total.value"
                />
            </template>
        </div>
    </div>
</template>
