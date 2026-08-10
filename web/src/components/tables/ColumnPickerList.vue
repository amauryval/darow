<script setup lang="ts">
import type { ColumnSchema } from '../../client/types'

// A column list where a click inserts the name instead of selecting a row —
// shared by SqlSchemaReference (one table's columns, expanded) and
// TransformColumnModal's free-SQL field (the table being transformed's
// columns, always in view since there is only ever one table to reference).
defineProps<{ columns: ColumnSchema[] }>()
const emit = defineEmits<{ insert: [name: string] }>()
</script>

<template>
    <button
        v-for="col in columns"
        :key="col.name"
        type="button"
        class="flex w-full items-center justify-between gap-2 truncate rounded px-1 py-0.5 text-left text-xs hover:bg-base-200"
        :title="`${col.name} · ${col.type}`"
        @click="emit('insert', col.name)"
    >
        <span class="truncate">{{ col.name }}</span>
        <span class="shrink-0 text-base-content/40">{{ col.type }}</span>
    </button>
</template>
