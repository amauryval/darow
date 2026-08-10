<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import { Plus } from '@lucide/vue'
import type { ColumnSchema } from '../../client/types'
import { useColumnTransform } from '../../composables/useColumnTransform'
import { useToast } from '../../composables/useToast'
import { isHiddenColumn } from '../../utils/rows'
import ConfirmDialog from '../ui/ConfirmDialog.vue'
import FilterNodeEditor from '../filters/FilterNodeEditor.vue'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import Typography from '../ui/Typography.vue'
import AssignmentEditor from './AssignmentEditor.vue'
import TransformCaseEditor from './TransformCaseEditor.vue'
import TransformPreview from './TransformPreview.vue'

// Rewrites column's values in place, on the rows a filter names — the write
// neither the one-cell-at-a-time edit session nor *Dériver* can express
// (spec/front/todo.md TODO-F44). Irreversible (§8.3): a preview against the
// server stands in for the undo the write does not have, and confirms
// before applying.
const props = defineProps<{
    tableId: string
    column: ColumnSchema
    columns: ColumnSchema[]
    geometryColumn?: string
}>()
const emit = defineEmits<{ close: []; transformed: [{ written: number }] }>()

const toast = useToast()
const {
    cases,
    defaultAssignment,
    conditions,
    conditionsBool,
    incomplete,
    addCase,
    removeCase,
    updateCaseConditions,
    updateCaseConditionsBool,
    updateCaseAssignment,
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
    computed(() => props.column.type),
    toRef(props, 'columns')
)

// Neither the target column nor the geometry can be an operand or a
// target — the same exclusion DeriveFromTableForm applies to what a
// derivation may carry over.
const operandColumns = computed(() =>
    props.columns.filter((c) => !isHiddenColumn(c.name) && c.name !== props.geometryColumn)
)

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3) — the same rule DeriveTableModal
// applies to its own draft.
const dirty = computed(
    () =>
        cases.value.length > 0 ||
        conditions.value.length > 0 ||
        defaultAssignment.value.kind !== 'constant' ||
        defaultAssignment.value.constant.trim() !== ''
)

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
        wide
        :dirty="dirty"
        @close="emit('close')"
        @submit="onPrimaryAction"
    >
        <div class="flex flex-col gap-4">
            <section>
                <Typography variant="eyebrow">Lignes concernées</Typography>
                <Typography variant="caption" class="mt-1 block"
                    >Sans condition, toutes les lignes sont concernées.</Typography
                >
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
                <Typography variant="eyebrow">Cas</Typography>
                <Typography variant="caption" class="mt-1 block"
                    >Le premier cas dont les conditions correspondent s'applique ; sans cas, « Sinon
                    » s'applique à toutes les lignes concernées.</Typography
                >
                <TransformCaseEditor
                    v-for="c in cases"
                    :key="c.id"
                    class="mt-2"
                    :table-id="tableId"
                    :target-column="column"
                    :columns="operandColumns"
                    :conditions="c.conditions"
                    :conditions-bool="c.conditionsBool"
                    :assignment="c.assignment"
                    @update:conditions="updateCaseConditions(c.id, $event)"
                    @update:conditions-bool="updateCaseConditionsBool(c.id, $event)"
                    @update:assignment="updateCaseAssignment(c.id, $event)"
                    @remove="removeCase(c.id)"
                />
                <BaseButton variant="ghost" size="xs" :icon="Plus" class="mt-2" @click="addCase"
                    >Ajouter un cas</BaseButton
                >
            </section>

            <section class="border-t border-base-300 pt-4">
                <Typography variant="eyebrow">Sinon</Typography>
                <AssignmentEditor
                    class="mt-2"
                    :table-id="tableId"
                    :target-column="column"
                    :columns="operandColumns"
                    :model-value="defaultAssignment"
                    @update:model-value="defaultAssignment = $event"
                />
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
                :title="incomplete ? 'Chaque cas a besoin d\'au moins une condition.' : undefined"
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
