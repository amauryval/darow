<script setup lang="ts">
import type { TransformSample } from '../../client/types'
import Skeleton from '../ui/Skeleton.vue'
import Typography from '../ui/Typography.vue'

// The four states of a column transform's preview (§8.1) — what stands in
// for the undo the write does not have (spec.md §5.11): the row count and a
// bounded before/after sample, or nothing yet asked for.
defineProps<{
    column: string
    loading: boolean
    error: string
    matched: number | null
    sample: TransformSample[]
}>()

function display(value: unknown): string {
    return value === null || value === undefined ? '—' : String(value)
}
</script>

<template>
    <div class="rounded-md border border-base-300 bg-base-200 px-3 py-2">
        <Skeleton v-if="loading" :rows="3" />
        <p v-else-if="error" class="text-xs text-error" role="alert">{{ error }}</p>
        <p v-else-if="matched === null" class="text-xs text-base-content/50">
            Lancez l'aperçu pour voir le nombre de lignes concernées.
        </p>
        <div v-else class="flex flex-col gap-2">
            <Typography variant="body-sm">
                {{ matched.toLocaleString('fr-FR') }} ligne{{ matched > 1 ? 's' : '' }} concernée{{
                    matched > 1 ? 's' : ''
                }}.
            </Typography>
            <div v-if="sample.length > 0" class="max-h-40 overflow-y-auto">
                <table class="table-xs table w-full">
                    <thead>
                        <tr class="text-xs text-base-content/60">
                            <th>Ligne</th>
                            <th>Avant</th>
                            <th>Après ({{ column }})</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="row in sample" :key="row.pkey">
                            <td class="whitespace-nowrap">{{ row.pkey }}</td>
                            <td class="truncate">{{ display(row.before) }}</td>
                            <td class="truncate">{{ display(row.after) }}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</template>
