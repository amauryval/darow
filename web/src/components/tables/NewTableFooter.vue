<script setup lang="ts">
import BaseInput from '../ui/BaseInput.vue'
import FormError from '../ui/FormError.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'
import Typography from '../ui/Typography.vue'

// The name field, its snapshot caption, the inline error and the footer
// DeriveTableModal and AggregateTableModal both close on: a new table's
// name, and the warning that it is a frozen snapshot, never resynchronised
// with its source. They differ only in the caption's exact wording and in
// canSubmit's own extra guard (`isEmpty` on the aggregation side), which
// stay the caller's.
defineProps<{
    name: string
    caption: string
    error: string
    confirmLabel: string
    busy: boolean
    disabled: boolean
    reason?: string
}>()
const emit = defineEmits<{
    'update:name': [name: string]
    cancel: []
    confirm: []
}>()
</script>

<template>
    <div class="mt-4 border-t border-base-300 pt-4">
        <BaseInput
            :model-value="name"
            size="sm"
            label="Nom de la nouvelle table"
            placeholder="Nom de la nouvelle table"
            @update:model-value="emit('update:name', $event)"
        />
        <Typography variant="caption" class="mt-1 block">{{ caption }}</Typography>
    </div>

    <FormError :message="error" />

    <ModalFormActions
        :confirm-label="confirmLabel"
        :busy="busy"
        :disabled="disabled"
        :reason="reason"
        @cancel="emit('cancel')"
        @confirm="emit('confirm')"
    />
</template>
