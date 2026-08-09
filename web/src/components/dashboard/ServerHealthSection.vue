<script setup lang="ts">
import { computed } from 'vue'
import { HardDrive, MemoryStick, Puzzle, Waves } from '@lucide/vue'
import type { Diagnostics } from '../../client/types'
import { formatSize } from '../../utils/sourceRows'
import Typography from '../ui/Typography.vue'
import StatTile from '../ui/StatTile.vue'
import Badge from '../ui/Badge.vue'
import Skeleton from '../ui/Skeleton.vue'
import ErrorState from '../ui/ErrorState.vue'
import BarChart from '../charts/BarChart.vue'
import DonutChart from '../charts/DonutChart.vue'

const props = defineProps<{
    diagnostics: Diagnostics | null
    loading: boolean
    loadedOnce: boolean
    error: string
}>()
defineEmits<{ retry: [] }>()

const metrics = computed(() =>
    props.diagnostics
        ? [
              { label: 'Base de données', value: props.diagnostics.databaseSizeBytes },
              { label: 'Mémoire', value: props.diagnostics.memoryUsageBytes },
              { label: 'Fichiers temporaires', value: props.diagnostics.temporaryFilesBytes }
          ]
        : []
)

// maxSizeBytes = 0 means the operator left duckdb.maxSizeBytes unbounded —
// there is no "remaining" to show against.
const sizeQuota = computed(() => {
    const d = props.diagnostics
    if (!d || d.maxSizeBytes <= 0) return null
    return [
        { label: 'Utilisé', value: d.databaseSizeBytes },
        { label: 'Restant', value: Math.max(d.maxSizeBytes - d.databaseSizeBytes, 0) }
    ]
})
</script>

<template>
    <section class="rounded-lg border border-base-300 bg-base-100 p-4">
        <Typography variant="h4" class="mb-4">Santé du serveur</Typography>

        <Skeleton v-if="loading && !loadedOnce" :rows="2" :columns="4" />
        <ErrorState v-else-if="error && !diagnostics" :message="error" @retry="$emit('retry')" />
        <div v-else-if="diagnostics" class="flex flex-col gap-4">
            <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
                <StatTile
                    label="Taille de la base"
                    :value="formatSize(diagnostics.databaseSizeBytes)"
                    :icon="HardDrive"
                />
                <StatTile
                    label="Mémoire utilisée"
                    :value="formatSize(diagnostics.memoryUsageBytes)"
                    :icon="MemoryStick"
                />
                <StatTile
                    label="Fichiers temporaires"
                    :value="formatSize(diagnostics.temporaryFilesBytes)"
                    :icon="Waves"
                />
                <StatTile
                    label="Extensions chargées"
                    :value="String(diagnostics.extensions.length)"
                    :icon="Puzzle"
                />
            </div>
            <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
                <BarChart :data="metrics" :value-format="formatSize" />
                <div v-if="sizeQuota">
                    <Typography variant="eyebrow" class="mb-2 block">
                        Espace disque ({{ formatSize(diagnostics.maxSizeBytes) }} max)
                    </Typography>
                    <DonutChart :data="sizeQuota" :value-format="formatSize" />
                </div>
            </div>
            <div class="flex flex-wrap gap-1.5">
                <Badge v-for="ext in diagnostics.extensions" :key="ext" tone="neutral">
                    {{ ext }}
                </Badge>
            </div>
        </div>
    </section>
</template>
