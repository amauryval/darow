<script setup lang="ts">
import { computed } from 'vue'
import { columnKind, typeError } from '../../utils/columnTypes'
import BaseInput from '../ui/BaseInput.vue'
import BaseSelect from '../ui/BaseSelect.vue'

// A cell of the grid while the edit mode is on. Every editable column is one:
// a session makes the whole table writable, not one row of it, so there is no
// per-row button to arm and none to close.
//
// The control follows the column's **type**, because a text box that accepts
// anything and a save that refuses the whole session an hour later is the
// worst of both: a boolean is a choice of three (vrai / faux / vide, since
// empty is NULL and not `false`), a number is a number field, a date is a
// date field, everything else is text checked for what its type can hold.
// What is still wrong is said on the cell that carries it, before the write.
//
// `dirty` marks the cell a save would write — amber, the colour a pending
// write carries everywhere, the same one the map draws a moved geometry in.
// The column names the field: a grid header is not an accessible name for the
// input under it (§8.7).
const props = defineProps<{ modelValue?: string; column: string; type: string; dirty?: boolean }>()
defineEmits<{ 'update:modelValue': [value: string] }>()

const kind = computed(() => columnKind(props.type))
const error = computed(() => typeError(props.type, props.modelValue ?? ''))

const BOOLEAN_OPTIONS = [
    { value: '', label: '—' },
    { value: 'true', label: 'Vrai' },
    { value: 'false', label: 'Faux' }
]

// A dirty cell is amber, a badly typed one red: one says "this will be
// written", the other "this cannot be", and the second wins when both hold.
const tint = computed(() => {
    if (error.value !== null) return '[&_input]:border-error [&_input]:bg-error/10'
    return props.dirty ? '[&_input]:border-warning [&_input]:bg-warning/10' : ''
})
</script>

<template>
    <BaseSelect
        v-if="kind === 'boolean'"
        :model-value="modelValue"
        :options="BOOLEAN_OPTIONS"
        size="xs"
        :aria-label="column"
        :class="dirty ? '[&_select]:border-warning [&_select]:bg-warning/10' : ''"
        @update:model-value="$emit('update:modelValue', $event)"
    />
    <BaseInput
        v-else
        :model-value="modelValue"
        :type="
            kind === 'date' ? 'date' : kind === 'integer' || kind === 'number' ? 'number' : 'text'
        "
        :inputmode="kind === 'integer' ? 'numeric' : kind === 'number' ? 'decimal' : undefined"
        size="xs"
        :aria-label="column"
        :title="error ?? undefined"
        class="min-w-32"
        :class="tint"
        @update:model-value="$emit('update:modelValue', $event)"
    />
</template>
