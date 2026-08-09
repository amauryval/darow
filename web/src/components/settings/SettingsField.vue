<script setup lang="ts">
import { computed } from 'vue'
import { RotateCcw } from '@lucide/vue'
import type { SettingsFieldSpec } from '../../utils/settingsFields'
import type { Settings } from '../../client/types'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import Typography from '../ui/Typography.vue'

// inherited/showInheritToggle: TableSettingsModal's own state — "inherited"
// is whether this field currently follows the global value rather than the
// number 0, and showInheritToggle is what draws the toggle at all (the
// global screen never passes it, so its own perTable fields show no
// toggle).
const props = withDefaults(
    defineProps<{
        field: SettingsFieldSpec
        modelValue: string
        settings: Settings | null
        error?: string
        disabled?: boolean
        inherited?: boolean
        showInheritToggle?: boolean
    }>(),
    { inherited: false, showInheritToggle: false }
)

defineEmits<{ 'update:modelValue': [value: string]; toggleInherit: [] }>()

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
        <div class="flex items-end gap-2">
            <BaseInput
                :model-value="modelValue"
                type="number"
                :inputmode="field.step ? 'decimal' : 'numeric'"
                :step="field.step"
                :label="field.unit ? `${field.label} (${field.unit})` : field.label"
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
