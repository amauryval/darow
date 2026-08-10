<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import type { ColumnSchema } from '../../client/types'
import { useColumnTransform } from '../../composables/useColumnTransform'
import { useSavedFilters } from '../../composables/useSavedFilters'
import { useToast } from '../../composables/useToast'
import { fromConditions } from '../../utils/filterTree'
import { isHiddenColumn } from '../../utils/rows'
import BaseSelect from '../ui/BaseSelect.vue'
import BaseTextarea from '../ui/BaseTextarea.vue'
import ColumnPickerList from './ColumnPickerList.vue'
import ConfirmDialog from '../ui/ConfirmDialog.vue'
import DocLink from '../ui/DocLink.vue'
import FilterNodeEditor from '../filters/FilterNodeEditor.vue'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import Typography from '../ui/Typography.vue'
import TransformPreview from './TransformPreview.vue'

const DUCKDB_FUNCTIONS_URL = 'https://duckdb.org/docs/sql/functions/overview'

// Rewrites column's values in place, on the rows a filter names — the write
// neither the one-cell-at-a-time edit session nor *Dériver* can express
// (spec/front/todo.md TODO-F44). Irreversible (§8.3): a preview against the
// server stands in for the undo the write does not have, and confirms
// before applying. The new value is a single free SQL expression rather
// than a guided arm picker — a per-row CASE WHEN builder was more machinery
// than the act needed; a value that must vary per condition is written as
// two transforms, each with its own filter.
const props = defineProps<{
    tableId: string
    column: ColumnSchema
    columns: ColumnSchema[]
    geometryColumn?: string
}>()
const emit = defineEmits<{ close: []; transformed: [{ written: number }] }>()

const toast = useToast()
const {
    expression,
    conditions,
    conditionsBool,
    incomplete,
    previewing,
    previewError,
    previewMatched,
    previewSample,
    runPreview,
    isLoading,
    error,
    submit
} = useColumnTransform(
    toRef(props, 'tableId'),
    computed(() => props.column.name),
    toRef(props, 'columns')
)

// Neither the target column nor the geometry can be an operand or a
// target — the same exclusion DeriveFromTableForm applies to what a
// derivation may carry over.
const operandColumns = computed(() =>
    props.columns.filter((c) => !isHiddenColumn(c.name) && c.name !== props.geometryColumn)
)

const { filters: savedFilters } = useSavedFilters(toRef(props, 'tableId'))
const savedFilterOptions = computed(() => [
    { value: '', label: 'Reprendre un filtre…' },
    ...savedFilters.value.map((f) => ({ value: f.filterId, label: f.name }))
])

// A saved filter is applied by copying its conditions into the draft, not by
// referencing it — the same rule DeriveFromTableForm applies: a later edit
// of the filter must not read as changing what rows an already-run transform
// touched.
function applySavedFilter(filterId: string) {
    const filter = savedFilters.value.find((f) => f.filterId === filterId)
    if (!filter) return
    conditions.value = fromConditions(filter.conditions, true)
}

const textarea = ref<InstanceType<typeof BaseTextarea> | null>(null)

function insert(text: string) {
    textarea.value?.insertAtCursor(text)
}

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3) — the same rule DeriveTableModal
// applies to its own draft.
const dirty = computed(() => conditions.value.length > 0 || expression.value.trim() !== '')

const confirming = ref(false)

async function onPrimaryAction() {
    if (incomplete.value) return
    if (previewMatched.value === null) {
        await runPreview()
        return
    }
    confirming.value = true
}

async function onConfirm() {
    const result = await submit()
    confirming.value = false
    if (result === null) return
    emit('transformed', result)
    emit('close')
    toast.success(`${result.written.toLocaleString('fr-FR')} ligne(s) mise(s) à jour.`)
}
</script>

<template>
    <Modal
        :title="`Mettre à jour « ${column.name} »`"
        xl
        :dirty="dirty"
        @close="emit('close')"
        @submit="onPrimaryAction"
    >
        <div class="flex flex-col gap-4">
            <section>
                <div class="mb-2 flex items-end justify-between gap-3">
                    <div>
                        <Typography variant="eyebrow">Condition</Typography>
                        <Typography variant="caption" class="mt-1 block"
                            >Sans condition, toutes les lignes sont concernées.</Typography
                        >
                    </div>
                    <BaseSelect
                        v-if="savedFilters.length > 0"
                        model-value=""
                        size="sm"
                        class="w-56"
                        aria-label="Reprendre un filtre enregistré"
                        :options="savedFilterOptions"
                        @update:model-value="applySavedFilter($event)"
                    />
                </div>
                <FilterNodeEditor
                    class="mt-2"
                    :table-id="tableId"
                    :columns="operandColumns"
                    :nodes="conditions"
                    :bool="conditionsBool"
                    @update:nodes="conditions = $event"
                    @update:bool="conditionsBool = $event"
                />
            </section>

            <section class="border-t border-base-300 pt-4">
                <Typography variant="eyebrow">Valeur</Typography>
                <Typography variant="caption" class="mt-1 block"
                    >Expression SQL calculée pour chaque ligne concernée.
                    <DocLink :href="DUCKDB_FUNCTIONS_URL">Fonctions DuckDB</DocLink></Typography
                >
                <div class="mt-2 flex gap-3">
                    <BaseTextarea
                        ref="textarea"
                        v-model="expression"
                        :rows="6"
                        class="flex-1"
                        aria-label="Expression SQL de la nouvelle valeur"
                    />
                    <div
                        class="h-40 w-48 shrink-0 overflow-y-auto rounded border border-base-300 p-1"
                    >
                        <ColumnPickerList :columns="operandColumns" @insert="insert" />
                    </div>
                </div>
            </section>

            <TransformPreview
                :column="column.name"
                :loading="previewing"
                :error="previewError"
                :matched="previewMatched"
                :sample="previewSample"
            />

            <p v-if="error" class="text-xs text-error" role="alert">{{ error }}</p>
        </div>

        <div class="mt-4 flex justify-end gap-2">
            <BaseButton variant="secondary" @click="emit('close')">Annuler</BaseButton>
            <BaseButton
                variant="primary"
                :busy="previewing"
                :disabled="incomplete || previewing"
                :title="
                    incomplete ? 'Renseignez une expression pour la nouvelle valeur.' : undefined
                "
                @click="onPrimaryAction"
            >
                {{ previewing ? 'Aperçu…' : previewMatched === null ? 'Aperçu' : 'Appliquer' }}
            </BaseButton>
        </div>

        <ConfirmDialog
            v-if="confirming"
            title="Confirmer la mise à jour"
            :message="`Mettre à jour « ${column.name} » sur ${(previewMatched ?? 0).toLocaleString(
                'fr-FR'
            )} ligne(s) ? Cette action est irréversible.`"
            confirm-label="Mettre à jour"
            :busy="isLoading"
            @confirm="onConfirm"
            @close="confirming = false"
        />
    </Modal>
</template>
