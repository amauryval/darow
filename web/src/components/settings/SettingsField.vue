<script setup lang="ts">
import { computed } from 'vue'
import { RotateCcw } from '@lucide/vue'
import type { FieldBounds } from '../../client/types'
import { inputRange, type SettingsFieldSpec } from '../../utils/settingsFields'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import Typography from '../ui/Typography.vue'

// bounds come from the payload the field was read in, never from the spec:
// the server is the only authority on what it accepts, and a bound retyped
// here is a bound that drifts from the one it is checked against.
//
// inherited/showInheritToggle: TableSettingsModal's own state — "inherited"
// is whether this field currently follows the global value rather than the
// number 0, and showInheritToggle is what draws the toggle at all (the
// global screen never passes it, so its own perTable fields show no
// toggle).
const props = withDefaults(
    defineProps<{
        field: SettingsFieldSpec
        modelValue: string
        bounds?: FieldBounds
        error?: string
        disabled?: boolean
        inherited?: boolean
        showInheritToggle?: boolean
    }>(),
    { inherited: false, showInheritToggle: false }
)

defineEmits<{ 'update:modelValue': [value: string]; toggleInherit: [] }>()

const range = computed(() => inputRange(props.bounds, props.field.step))
const label = computed(() =>
    props.bounds?.unit ? `${props.field.label} (${props.bounds.unit})` : props.field.label
)
const rangeHint = computed(() => {
    const { min, max } = range.value
    if (min === undefined && max === undefined) return ''
    if (max === undefined) return `Minimum ${min}`
    if (min === undefined) return `Maximum ${max}`
    return `Entre ${min} et ${max}`
})
</script>

<template>
    <div>
        <div class="flex items-end gap-2">
            <BaseInput
                :model-value="modelValue"
                type="number"
                :inputmode="field.step ? 'decimal' : 'numeric'"
                :step="field.step"
                :min="range.min"
                :max="range.max"
                :label="label"
                :error="error"
                :disabled="disabled || inherited"
                class="flex-1"
                @update:model-value="$emit('update:modelValue', $event)"
            />
            <BaseButton
                v-if="showInheritToggle"
                type="button"
                variant="ghost"
                size="xs"
                :icon="RotateCcw"
                icon-only
                :disabled="disabled"
                :title="
                    inherited ? 'Personnaliser pour cette table' : 'Revenir à la valeur globale'
                "
                @click="$emit('toggleInherit')"
            >
                {{ inherited ? 'Personnaliser' : 'Hériter' }}
            </BaseButton>
        </div>
        <Typography variant="caption" class="mt-1 block">{{ field.description }}</Typography>
        <Typography v-if="rangeHint" variant="caption" class="mt-1 block text-base-content/60">
            {{ rangeHint }}
        </Typography>
        <Typography
            v-if="field.zeroMeaning && Number(modelValue) === 0"
            variant="caption"
            class="mt-1 block text-base-content/60"
        >
            0 = {{ field.zeroMeaning }}
        </Typography>
    </div>
</template>
