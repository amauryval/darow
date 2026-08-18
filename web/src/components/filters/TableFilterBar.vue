<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import { Filter } from '@lucide/vue'
import { errorMessage } from '../../client/http'
import type { ColumnSchema, SavedFilter } from '../../client/types'
import { useSavedFilters } from '../../composables/useSavedFilters'
import { useToast } from '../../composables/useToast'
import { coerceByType, columnType } from '../../utils/columnTypes'
import { fromConditions, nodeOf, pruneActive, type DraftNode } from '../../utils/filterTree'
import FilterEditorModal from './FilterEditorModal.vue'
import ActiveFilterPill from './ActiveFilterPill.vue'
import SavedFilterMenu from './SavedFilterMenu.vue'
import BaseButton from '../ui/BaseButton.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// activeFilterId is owned by the view (TableDetailView, §9 — the URL carries
// the applied filter) and controlled here through the usual prop/emit pair,
// so the reset-on-tableId-change watch below still assigns to it directly.
const props = defineProps<{
    tableId: string
    columns: ColumnSchema[]
    activeFilterId: string | null
}>()
// The tree the parent filters on is derived here, never held twice: every
// filter is persisted, so the catalogue is the single source of truth and the
// bar only tracks which of its filters is applied.
const emit = defineEmits<{
    'update:nodes': [nodes: DraftNode[]]
    'update:activeFilterId': [id: string | null]
}>()

const toast = useToast()
const saved = useSavedFilters(toRef(props, 'tableId'))
const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

// Only one filter applies at a time: a second one would silently AND with the
// first, which the toolbar cannot show.
const activeFilterId = computed({
    get: () => props.activeFilterId,
    set: (id) => emit('update:activeFilterId', id)
})
const activeFilter = computed(
    () => saved.filters.value.find((f) => f.filterId === activeFilterId.value) ?? null
)

watch(
    [saved.filters, activeFilterId],
    () =>
        emit(
            'update:nodes',
            activeFilter.value ? fromConditions(activeFilter.value.conditions, true) : []
        ),
    { deep: true }
)

// A table change resets the applied filter: the ids belong to the table that
// was left behind.
watch(
    () => props.tableId,
    () => (activeFilterId.value = null)
)

function conditionsOf(node: DraftNode) {
    return pruneActive([node], (col, value) => coerceByType(columnType(props.columns, col), value))
}

// editing.filter null = creating; otherwise the stored filter being edited,
// which is what makes "Appliquer" update it rather than create a duplicate.
const editing = ref<{ filter: SavedFilter | null } | null>(null)
const editorError = ref('')

const editedNode = computed(() =>
    editing.value?.filter
        ? nodeOf('AND', fromConditions(editing.value.filter.conditions, true), true)
        : null
)

function openCreate() {
    editorError.value = ''
    editing.value = { filter: null }
}

function openEdit(filter: SavedFilter) {
    editorError.value = ''
    editing.value = { filter }
}

function toggleFilter(filter: SavedFilter) {
    activeFilterId.value = activeFilterId.value === filter.filterId ? null : filter.filterId
}

async function onSubmit(payload: { node: DraftNode; name: string }) {
    editorError.value = ''
    const target = editing.value?.filter
    try {
        const stored = target
            ? await saved.update(target.filterId, {
                  name: payload.name,
                  conditions: conditionsOf(payload.node)
              })
            : await saved.create(payload.name, conditionsOf(payload.node))
        activeFilterId.value = stored.filterId
        editing.value = null
        toast.success(
            target
                ? `Filtre « ${payload.name} » mis à jour.`
                : `Filtre « ${payload.name} » enregistré.`
        )
    } catch (e) {
        // The modal stays open on its own error rather than a toast: the user has
        // to act on it, and their conditions are still in front of them (§8.4).
        editorError.value = errorMessage(e, "Impossible d'enregistrer le filtre")
    }
}

async function onRemove(filter: SavedFilter) {
    try {
        await saved.remove(filter.filterId)
        if (activeFilterId.value === filter.filterId) activeFilterId.value = null
        toast.success(`Filtre « ${filter.name} » supprimé.`)
    } catch (e) {
        toast.error(errorMessage(e, 'Impossible de supprimer le filtre'))
    }
}
</script>

<template>
    <div class="flex min-w-0 flex-wrap items-center gap-2">
        <BaseButton
            variant="primary"
            size="sm"
            :icon="Filter"
            :disabled="writeDisabled"
            :title="writeDisabled ? writeGuardReason : undefined"
            @click="openCreate"
            >Filtrer</BaseButton
        >

        <SavedFilterMenu
            :filters="saved.filters.value"
            :active-filter-id="activeFilterId"
            :loading="saved.loading.value"
            :loaded-once="saved.loadedOnce.value"
            :error="saved.error.value"
            @apply="toggleFilter"
            @edit="openEdit"
            @remove="onRemove"
            @retry="saved.load"
        />

        <ActiveFilterPill
            v-if="activeFilter"
            :filter="activeFilter"
            @clear="activeFilterId = null"
            @edit="openEdit(activeFilter)"
        />

        <FilterEditorModal
            v-if="editing"
            :key="editing.filter?.filterId ?? 'new'"
            :table-id="tableId"
            :columns="columns"
            :node="editedNode"
            :saved-name="editing.filter?.name"
            :saving="saved.saving.value"
            :error="editorError"
            @submit="onSubmit"
            @close="editing = null"
        />
    </div>
</template>
