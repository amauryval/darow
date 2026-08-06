<script setup lang="ts">
import { computed } from 'vue'
import { Database, Layers, MapPin, Rows3 } from '@lucide/vue'
import type { DashboardStats } from '../../utils/dashboardStats'
import Typography from '../ui/Typography.vue'
import StatTile from '../ui/StatTile.vue'
import Skeleton from '../ui/Skeleton.vue'
import ErrorState from '../ui/ErrorState.vue'
import BarChart from '../charts/BarChart.vue'
import DonutChart from '../charts/DonutChart.vue'

const props = defineProps<{
    stats: DashboardStats | null
    loading: boolean
    loadedOnce: boolean
    error: string
}>()
defineEmits<{ retry: [] }>()

const geoSplit = computed(() =>
    props.stats
        ? [
              { label: 'Géographiques', value: props.stats.geoTableCount },
              { label: 'Non géographiques', value: props.stats.nonGeoTableCount }
          ]
        : []
)

const sridSplit = computed(
    () =>
        props.stats?.sridDistribution.map((s) => ({ label: `EPSG:${s.srid}`, count: s.count })) ??
        []
)

const topTables = computed(
    () => props.stats?.topTablesByRows.map((t) => ({ label: t.tableName, value: t.rowCount })) ?? []
)
</script>

<template>
    <section class="rounded-lg border border-base-300 bg-base-100 p-4">
        <Typography variant="h4">Vue d'ensemble des données</Typography>
        <Typography variant="body-sm" class="mt-1 mb-4 text-base-content/60">
            Ce que le catalogue de tables contient aujourd'hui.
        </Typography>

        <Skeleton v-if="loading && !loadedOnce" :rows="2" :columns="4" />
        <ErrorState v-else-if="error && !stats" :message="error" @retry="$emit('retry')" />
        <div v-else-if="stats" class="flex flex-col gap-6">
            <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
                <StatTile label="Tables" :value="String(stats.tableCount)" :icon="Database" />
                <StatTile
                    label="Lignes au total"
                    :value="stats.totalRows.toLocaleString('fr-FR')"
                    :icon="Rows3"
                />
                <StatTile
                    label="Tables géographiques"
                    :value="String(stats.geoTableCount)"
                    :icon="MapPin"
                />
                <StatTile
                    label="Tables non géographiques"
                    :value="String(stats.nonGeoTableCount)"
                    :icon="Layers"
                />
            </div>

            <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
                <div>
                    <Typography variant="eyebrow" class="mb-2 block">Géo vs non géo</Typography>
                    <DonutChart :data="geoSplit" />
                </div>
                <div>
                    <Typography variant="eyebrow" class="mb-2 block">
                        Répartition par SRID
                    </Typography>
                    <DonutChart
                        v-if="sridSplit.length > 0"
                        :data="sridSplit.map((s) => ({ label: s.label, value: s.count }))"
                    />
                    <Typography v-else variant="body-sm" class="text-base-content/60">
                        Aucune table géographique.
                    </Typography>
                </div>
            </div>

            <div>
                <Typography variant="eyebrow" class="mb-2 block">
                    Tables les plus volumineuses
                </Typography>
                <BarChart
                    v-if="topTables.length > 0"
                    :data="topTables"
                    :value-format="(n) => n.toLocaleString('fr-FR')"
                />
                <Typography v-else variant="body-sm" class="text-base-content/60">
                    Aucune table.
                </Typography>
            </div>
        </div>
    </section>
</template>
