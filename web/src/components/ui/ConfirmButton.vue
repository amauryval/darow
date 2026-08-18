<script setup lang="ts">
import { ref } from 'vue'
import type { Component } from 'vue'
import BaseButton from './BaseButton.vue'
import ConfirmDialog from './ConfirmDialog.vue'

// A destructive action and its confirmation in one control: the trigger here,
// the wording in the shared `ConfirmDialog` — consistent across a single rare
// action (delete a table) and a frequent one (delete a row) alike (§8.3).
const props = withDefaults(
    defineProps<{
        label: string
        confirmLabel?: string
        // Names what is about to disappear and any cascade effect; falls back to
        // a generic phrasing built from `label` when the caller has nothing more
        // specific to say.
        message?: string
        busy?: boolean
        // Forwarded to the trigger so it matches the size and the icon language
        // of the buttons it stands with — a delete that is a size apart from its
        // neighbours reads as a foreign control (§7).
        size?: 'xs' | 'sm' | 'md'
        icon?: Component
        // The icon alone, `label` becoming its accessible name and its tooltip:
        // a grid row states the same action once per line, and a red pill on
        // every one of them reads as a warning about the data rather than a
        // control (§7). The confirmation is unchanged — it is where the wording
        // has to be.
        iconOnly?: boolean
        // Filled red is for a button that stands alone; among a row's icon
        // actions the destructive one is a ghost that turns red under the
        // pointer, so the grid is not striped with alarm colour.
        variant?: 'danger' | 'ghost'
        // Plain trigger-disabling, same contract as BaseButton's — the caller
        // (a view or a domain component reading useWriteGuard) decides whether
        // the action is refused; this component stays domain-free and only
        // renders the verdict it is handed.
        disabled?: boolean
        title?: string
    }>(),
    { confirmLabel: 'Confirmer', size: 'xs', variant: 'danger' }
)

const emit = defineEmits<{ confirm: [] }>()

const open = ref(false)

defineExpose({ reset: () => (open.value = false) })
</script>

<template>
    <BaseButton
        :variant="variant"
        :size="size"
        :icon="icon"
        :icon-only="iconOnly"
        :disabled="disabled"
        :aria-label="iconOnly ? label : undefined"
        :title="title ?? (iconOnly ? label : undefined)"
        :class="variant === 'ghost' ? 'text-error/70 hover:text-error' : ''"
        @click="open = true"
    >
        {{ label }}
    </BaseButton>
    <ConfirmDialog
        v-if="open"
        :message="props.message ?? `${props.label} ? Cette action est irréversible.`"
        :confirm-label="props.confirmLabel"
        :busy="props.busy"
        @confirm="emit('confirm')"
        @close="open = false"
    />
</template>
