<script setup lang="ts">
import BaseButton from './BaseButton.vue'

// The footer every submitting modal ends on: a secondary "Annuler" and the
// primary action, its own busy/disabled state and the reason it is disabled
// when there is one. The confirm label is the caller's — it is the caller
// that knows whether busy swaps it for a participle (§8.2) or, as a load
// that replaces rather than creates does, does not.
defineProps<{
    confirmLabel: string
    busy?: boolean
    disabled?: boolean
    reason?: string
}>()
const emit = defineEmits<{ cancel: []; confirm: [] }>()
</script>

<template>
    <div class="mt-4 flex justify-end gap-2">
        <BaseButton variant="secondary" @click="emit('cancel')">Annuler</BaseButton>
        <BaseButton
            variant="primary"
            :busy="busy"
            :disabled="disabled"
            :title="reason"
            @click="emit('confirm')"
        >
            {{ confirmLabel }}
        </BaseButton>
    </div>
</template>
