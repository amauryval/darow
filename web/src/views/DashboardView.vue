<script setup lang="ts">
import { computed } from 'vue'
import { useDiagnostics } from '../composables/useDiagnostics'
import { useDashboardStats } from '../composables/useDashboardStats'
import { useDocumentTitle } from '../composables/useDocumentTitle'
import Breadcrumbs from '../components/ui/Breadcrumbs.vue'
import Skeleton from '../components/ui/Skeleton.vue'
import ServerHealthSection from '../components/dashboard/ServerHealthSection.vue'
import DataOverviewSection from '../components/dashboard/DataOverviewSection.vue'

useDocumentTitle(computed(() => 'Tableau de bord'))

const diagnostics = useDiagnostics()
const dashboardStats = useDashboardStats()

// One shared skeleton for the whole page while both regions are on their
// first load; once either has loaded once, each section owns its own
// error/content state independently (§8.1) — a diagnostics failure never
// blocks the data-overview section, and vice versa.
const firstLoading = computed(
    () =>
        (diagnostics.loading.value && !diagnostics.loadedOnce.value) ||
        (dashboardStats.loading.value && !dashboardStats.loadedOnce.value)
)
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col gap-4 p-6">
        <Breadcrumbs :items="[{ label: 'Tableau de bord' }]" class="shrink-0" />

        <Skeleton v-if="firstLoading" :rows="4" :columns="4" />
        <div v-else class="min-h-0 flex-1 overflow-y-auto">
            <div class="flex flex-col gap-6 pb-2">
                <ServerHealthSection
                    :diagnostics="diagnostics.data.value"
                    :loading="diagnostics.loading.value"
                    :loaded-once="diagnostics.loadedOnce.value"
                    :error="diagnostics.error.value"
                    @retry="diagnostics.retry"
                />
                <DataOverviewSection
                    :stats="dashboardStats.stats.value"
                    :loading="dashboardStats.loading.value"
                    :loaded-once="dashboardStats.loadedOnce.value"
                    :error="dashboardStats.error.value"
                    @retry="dashboardStats.retry"
                />
            </div>
        </div>
    </div>
</template>
