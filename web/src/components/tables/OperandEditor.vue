<script setup lang="ts">
import { computed } from 'vue'
import type { ColumnSchema } from '../../client/types'
import type { OperandDraft } from '../../utils/transformAssignment'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'

// One operand of a concatenation or an arithmetic operation: a reference to
// another column of the row, or a literal value typed in. columns is
// already narrowed by the caller to what an operand may legitimately name
// (excluding the geometry column and the hidden ones).
const props = defineProps<{ modelValue: OperandDraft; columns: ColumnSchema[] }>()
const emit = defineEmits<{ 'update:modelValue': [draft: OperandDraft] }>()

const kindOptions = [
    { value: 'literal', label: 'Valeur' },
    { value: 'column', label: 'Colonne' }
]

const columnOptions = computed(() => props.columns.map((c) => ({ value: c.name, label: c.name })))

function updateKind(kind: string) {
    emit('update:modelValue', {
        ...props.modelValue,
        kind: kind === 'column' ? 'column' : 'literal'
    })
}
</script>

<template>
    <div class="flex items-center gap-1.5">
        <BaseSelect
            :model-value="modelValue.kind"
            size="sm"
            class="w-28 shrink-0"
            aria-label="Type d'opérande"
            :options="kindOptions"
            @update:model-value="updateKind"
        />
        <BaseSelect
            v-if="modelValue.kind === 'column'"
            :model-value="modelValue.column"
            size="sm"
            class="min-w-0 flex-1"
            aria-label="Colonne"
            :options="columnOptions"
            @update:model-value="emit('update:modelValue', { ...modelValue, column: $event })"
        />
        <BaseInput
            v-else
            :model-value="modelValue.literal"
            size="sm"
            class="min-w-0 flex-1"
            aria-label="Valeur littérale"
            placeholder="Valeur"
            @update:model-value="emit('update:modelValue', { ...modelValue, literal: $event })"
        />
    </div>
</template>
