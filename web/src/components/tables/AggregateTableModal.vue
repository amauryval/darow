<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import type { ColumnSchema } from '../../client/types'
import { useAggregateFromTable } from '../../composables/useAggregateFromTable'
import { useSavedFilters } from '../../composables/useSavedFilters'
import { useToast } from '../../composables/useToast'
import Modal from '../ui/Modal.vue'
import DeriveAggregateForm from './DeriveAggregateForm.vue'
import NewTableFooter from './NewTableFooter.vue'

// The aggregation mode's own screen (TODO-F37) — not a third tab on
// DeriveTableModal, whose column picker means "carry this column over" and
// would mean "group by this column" here: the same word for two different
// acts is exactly the confusion the guided mode was built to remove.
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
    groupBy,
    aggregates,
    conditions,
    conditionsBool,
    incompleteAggregate,
    isEmpty,
    toggleGroupBy,
    addAggregate,
    updateAggregate,
    removeAggregate,
    submit
} = useAggregateFromTable(toRef(props, 'tableId'), toRef(props, 'columns'))
const { filters: savedFilters } = useSavedFilters(toRef(props, 'tableId'))

const name = ref('')

const canSubmit = computed(
    () =>
        name.value.trim() !== '' && !isLoading.value && !incompleteAggregate.value && !isEmpty.value
)

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3).
const dirty = computed(
    () =>
        name.value.trim() !== '' ||
        groupBy.value.length > 0 ||
        aggregates.value.length > 0 ||
        conditions.value.length > 0
)

async function onSubmit() {
    if (!canSubmit.value) return
    const result = await submit(name.value.trim())
    // A null result means the call failed and `error` already carries the
    // message — it stays in front of the user rather than going out as a
    // toast (§8.4).
    if (result === null) return

    emit('derived', result)
    emit('close')
    toast.success('Table agrégée créée.')
}
</script>

<template>
    <Modal title="Agréger une table" wide :dirty="dirty" @close="emit('close')" @submit="onSubmit">
        <DeriveAggregateForm
            :table-id="tableId"
            :columns="columns"
            :geometry-column="geometryColumn"
            :group-by="groupBy"
            :aggregates="aggregates"
            :conditions="conditions"
            :conditions-bool="conditionsBool"
            :saved-filters="savedFilters"
            @toggle-group-by="toggleGroupBy"
            @add-aggregate="addAggregate"
            @update-aggregate="updateAggregate"
            @remove-aggregate="removeAggregate"
            @update:conditions="conditions = $event"
            @update:conditions-bool="conditionsBool = $event"
        />

        <NewTableFooter
            :name="name"
            caption="La table agrégée est un instantané figé : elle n'est jamais resynchronisée avec sa source."
            :error="error"
            :confirm-label="isLoading ? 'Création…' : 'Créer'"
            :busy="isLoading"
            :disabled="!canSubmit"
            :reason="
                incompleteAggregate
                    ? 'Complétez chaque colonne agrégée.'
                    : isEmpty
                      ? 'Sélectionnez au moins un regroupement ou un agrégat.'
                      : undefined
            "
            @update:name="name = $event"
            @cancel="emit('close')"
            @confirm="onSubmit"
        />
    </Modal>
</template>
