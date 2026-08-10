<script setup lang="ts">
import { computed, onMounted, toRef } from 'vue'
import { Plus, X } from '@lucide/vue'
import type { ColumnSchema } from '../../client/types'
import { useColumnDistinctValues } from '../../composables/useColumnDistinctValues'
import { columnKind } from '../../utils/columnTypes'
import {
    assignmentArms,
    createOperandDraft,
    type AssignmentDraft,
    type OperandDraft
} from '../../utils/transformAssignment'
import BaseButton from '../ui/BaseButton.vue'
import BaseAutocomplete from '../ui/BaseAutocomplete.vue'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'
import OperandEditor from './OperandEditor.vue'
import Typography from '../ui/Typography.vue'

// What a case (or the default) writes: a constant, a concatenation, an
// arithmetic operation or a free expression — the arms offered follow the
// target column's type (§0.2's spirit: an inapplicable arm is made
// unrepresentable rather than left for the server to refuse).
const props = defineProps<{
    tableId: string
    targetColumn: ColumnSchema
    columns: ColumnSchema[]
    modelValue: AssignmentDraft
}>()
const emit = defineEmits<{ 'update:modelValue': [draft: AssignmentDraft] }>()

const arms = computed(() => assignmentArms(columnKind(props.targetColumn.type)))
const armLabels: Record<string, string> = {
    constant: 'Valeur constante',
    concat: 'Concaténation',
    arithmetic: 'Opération',
    expression: 'Expression libre'
}
const armOptions = computed(() => arms.value.map((a) => ({ value: a, label: armLabels[a] })))

function update(patch: Partial<AssignmentDraft>) {
    emit('update:modelValue', { ...props.modelValue, ...patch })
}

// BaseSelect emits a bare string; narrowed against the arms this column
// type actually offers rather than asserted, so a value from outside that
// set (there is none today, but the type checker cannot know that) is
// silently ignored instead of forced through.
function updateKind(kind: string) {
    const found = arms.value.find((a) => a === kind)
    if (found) update({ kind: found })
}

// Existing values of the column being written, offered as suggestions for
// the constant arm — assigning against a value already in the column beats
// retyping it.
const distinct = useColumnDistinctValues(toRef(props, 'tableId'))
onMounted(() => distinct.toggle(props.targetColumn.name))
const distinctOptions = computed(() =>
    distinct.values.value.filter((v): v is string => v !== null).map(String)
)

function addConcatOperand() {
    update({ concat: [...props.modelValue.concat, createOperandDraft()] })
}
function updateConcatOperand(index: number, draft: OperandDraft) {
    update({ concat: props.modelValue.concat.map((o, i) => (i === index ? draft : o)) })
}
function removeConcatOperand(index: number) {
    update({ concat: props.modelValue.concat.filter((_, i) => i !== index) })
}
</script>

<template>
    <div class="flex flex-col gap-2">
        <BaseSelect
            :model-value="modelValue.kind"
            size="sm"
            class="w-48"
            aria-label="Type de valeur assignée"
            :options="armOptions"
            @update:model-value="updateKind"
        />

        <BaseAutocomplete
            v-if="modelValue.kind === 'constant'"
            :model-value="modelValue.constant"
            :options="distinctOptions"
            :loading="distinct.loading.value"
            size="sm"
            placeholder="Valeur"
            aria-label="Valeur constante"
            @update:model-value="update({ constant: $event })"
        />

        <div v-else-if="modelValue.kind === 'concat'" class="flex flex-col gap-1.5">
            <div
                v-for="(operand, index) in modelValue.concat"
                :key="index"
                class="flex items-center gap-1.5"
            >
                <OperandEditor
                    :model-value="operand"
                    :columns="columns"
                    class="flex-1"
                    @update:model-value="updateConcatOperand(index, $event)"
                />
                <BaseButton
                    variant="ghost"
                    size="xs"
                    square
                    :icon="X"
                    icon-only
                    aria-label="Supprimer cet opérande"
                    @click="removeConcatOperand(index)"
                />
            </div>
            <BaseButton variant="ghost" size="xs" :icon="Plus" @click="addConcatOperand"
                >Ajouter un opérande</BaseButton
            >
        </div>

        <div v-else-if="modelValue.kind === 'arithmetic'" class="flex items-center gap-1.5">
            <OperandEditor
                :model-value="modelValue.arithmeticLeft"
                :columns="columns"
                class="flex-1"
                @update:model-value="update({ arithmeticLeft: $event })"
            />
            <BaseSelect
                :model-value="modelValue.arithmeticOperator"
                size="sm"
                class="w-16 shrink-0"
                aria-label="Opérateur"
                :options="[
                    { value: '+', label: '+' },
                    { value: '-', label: '−' },
                    { value: '*', label: '×' },
                    { value: '/', label: '÷' }
                ]"
                @update:model-value="update({ arithmeticOperator: $event })"
            />
            <OperandEditor
                :model-value="modelValue.arithmeticRight"
                :columns="columns"
                class="flex-1"
                @update:model-value="update({ arithmeticRight: $event })"
            />
        </div>

        <template v-else-if="modelValue.kind === 'expression'">
            <BaseInput
                :model-value="modelValue.expression"
                size="sm"
                class="font-mono"
                placeholder="Expression SQL"
                aria-label="Expression SQL"
                @update:model-value="update({ expression: $event })"
            />
            <Typography variant="caption"
                >Postée telle quelle, sans validation côté client.</Typography
            >
        </template>
    </div>
</template>
