<script setup lang="ts">
import { RotateCcw } from '@lucide/vue'
import type { SettingsFieldSpec } from '../../utils/settingsFields'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import Typography from '../ui/Typography.vue'

// inherited/showInheritToggle: TableSettingsModal's own state — "inherited"
// is whether this field currently follows the global value rather than the
// number 0, and showInheritToggle is what draws the toggle at all (the
// global screen never passes it, so its own perTable fields show no
// toggle).
withDefaults(
    defineProps<{
        field: SettingsFieldSpec
        modelValue: string
        error?: string
        disabled?: boolean
        inherited?: boolean
        showInheritToggle?: boolean
    }>(),
    { inherited: false, showInheritToggle: false }
)

defineEmits<{ 'update:modelValue': [value: string]; toggleInherit: [] }>()
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
    </div>
</template>
