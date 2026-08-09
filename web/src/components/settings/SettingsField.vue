<script setup lang="ts">
import { computed } from 'vue'
import type { SettingsFieldSpec } from '../../utils/settingsFields'
import type { Settings } from '../../client/types'
import BaseInput from '../ui/BaseInput.vue'
import Typography from '../ui/Typography.vue'

const props = defineProps<{
    field: SettingsFieldSpec
    modelValue: string
    settings: Settings | null
    error?: string
    disabled?: boolean
}>()

defineEmits<{ 'update:modelValue': [value: string] }>()

// What the server applies today for this family, whatever the input holds:
// its own built-in values while the limit is left at 0, the operator's value
// once set. Read off the response, never recomputed here.
const effective = computed(() => {
    const keys = props.field.effectiveKeys
    if (!keys || !props.settings) return null
    return {
        default: props.settings[keys.default],
        max: props.settings[keys.max],
        noun: keys.noun
    }
})
</script>

<template>
    <div>
        <BaseInput
            :model-value="modelValue"
            type="number"
            :inputmode="field.step ? 'decimal' : 'numeric'"
            :step="field.step"
            :label="field.unit ? `${field.label} (${field.unit})` : field.label"
            :error="error"
            :disabled="disabled"
            @update:model-value="$emit('update:modelValue', $event)"
        />
        <Typography variant="caption" class="mt-1 block">{{ field.description }}</Typography>
        <Typography
            v-if="field.zeroMeaning && Number(modelValue) === 0"
            variant="caption"
            class="mt-1 block text-base-content/60"
        >
            0 = {{ field.zeroMeaning }}
        </Typography>
        <Typography v-if="effective" variant="caption" class="mt-1 block text-base-content/60">
            Actuellement : {{ effective.default }} {{ effective.noun }} par requête,
            {{ effective.max }} au maximum.
        </Typography>
    </div>
</template>
