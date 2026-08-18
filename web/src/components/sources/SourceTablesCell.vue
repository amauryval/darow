<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import type { TableProvenance } from '../../client/types'
import Typography from '../ui/Typography.vue'

// What a row's source already produced: the whole point of the column is that
// a second load is a deliberate choice, not a surprise.
const props = defineProps<{ tables: TableProvenance[] }>()

const MAX_SHOWN = 2

const shown = computed(() => props.tables.slice(0, MAX_SHOWN))
const extra = computed(() => props.tables.length - shown.value.length)
</script>

<template>
    <div v-if="tables.length > 0" class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <RouterLink
            v-for="table in shown"
            :key="table.tableId"
            :to="`/tables/${table.tableId}`"
            class="link link-primary truncate text-sm no-underline"
            @click.stop
            >{{ table.tableName }}</RouterLink
        >
        <Typography v-if="extra > 0" variant="caption">+{{ extra }}</Typography>
    </div>
    <Typography v-else variant="caption">—</Typography>
</template>
