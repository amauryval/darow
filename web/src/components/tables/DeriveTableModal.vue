<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import type { ColumnSchema } from '../../client/types'
import { useDeriveTable } from '../../composables/useDeriveTable'
import { useSavedFilters } from '../../composables/useSavedFilters'
import { useToast } from '../../composables/useToast'
import Modal from '../ui/Modal.vue'
import DeriveFromTableForm from './DeriveFromTableForm.vue'
import NewTableFooter from './NewTableFooter.vue'

// The guided mode alone: derives from the table in hand, offering its
// columns and reusing the filter editor for its conditions. The free-SQL
// mode used to be a second tab here, pre-filled with this table's name — a
// scope the endpoint never had, since it reads every registered table. It now
// lives in its own modal opened from the catalogue screen (`DeriveSqlModal`,
// TODO-F38).
const props = defineProps<{
    tableId: string
    columns: ColumnSchema[]
    geometryColumn?: string
}>()
const emit = defineEmits<{ close: []; derived: [{ tableId: string }] }>()

const toast = useToast()
const {
    isLoading,
    error,
    selectedColumns,
    computedColumns,
    conditions,
    conditionsBool,
    incompleteComputed,
    toggleColumn,
    addComputed,
    updateComputed,
    removeComputed,
    submitSpec
} = useDeriveTable(toRef(props, 'tableId'), toRef(props, 'columns'))
const { filters: savedFilters } = useSavedFilters(toRef(props, 'tableId'))

const name = ref('')

const canSubmit = computed(
    () => name.value.trim() !== '' && !isLoading.value && !incompleteComputed.value
)

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3).
const dirty = computed(
    () =>
        name.value.trim() !== '' ||
        selectedColumns.value.length > 0 ||
        computedColumns.value.length > 0 ||
        conditions.value.length > 0
)

async function onSubmit() {
    if (!canSubmit.value) return
    const result = await submitSpec(name.value.trim())
    // A null result means the call failed and `error` already carries the
    // message — it stays in front of the user rather than going out as a
    // toast (§8.4).
    if (result === null) return

    emit('derived', result)
    emit('close')
    toast.success('Table dérivée créée.')
}
</script>

<template>
    <Modal title="Dériver une table" wide :dirty="dirty" @close="emit('close')" @submit="onSubmit">
        <DeriveFromTableForm
            :table-id="tableId"
            :columns="columns"
            :geometry-column="geometryColumn"
            :selected-columns="selectedColumns"
            :computed-columns="computedColumns"
            :conditions="conditions"
            :conditions-bool="conditionsBool"
            :saved-filters="savedFilters"
            @toggle-column="toggleColumn"
            @add-computed="addComputed"
            @update-computed="updateComputed"
            @remove-computed="removeComputed"
            @update:conditions="conditions = $event"
            @update:conditions-bool="conditionsBool = $event"
        />

        <NewTableFooter
            :name="name"
            caption="La table dérivée est un instantané figé : elle n'est jamais resynchronisée avec sa source."
            :error="error"
            :confirm-label="isLoading ? 'Création…' : 'Créer'"
            :busy="isLoading"
            :disabled="!canSubmit"
            :reason="incompleteComputed ? 'Complétez chaque colonne calculée.' : undefined"
            @update:name="name = $event"
            @cancel="emit('close')"
            @confirm="onSubmit"
        />
    </Modal>
</template>
