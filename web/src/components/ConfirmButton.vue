<script setup lang="ts">
import { ref } from "vue";
import type { Component } from "vue";
import BaseButton from "./ui/BaseButton.vue";
import ConfirmDialog from "./ui/ConfirmDialog.vue";

// A destructive action and its confirmation in one control: the trigger here,
// the wording in the shared `ConfirmDialog` — consistent across a single rare
// action (delete a table) and a frequent one (delete a row) alike (§8.3).
const props = withDefaults(
  defineProps<{
    label: string;
    confirmLabel?: string;
    // Names what is about to disappear and any cascade effect; falls back to
    // a generic phrasing built from `label` when the caller has nothing more
    // specific to say.
    message?: string;
    busy?: boolean;
    // Forwarded to the trigger so it matches the size and the icon language
    // of the buttons it stands with — a delete that is a size apart from its
    // neighbours reads as a foreign control (§7).
    size?: "xs" | "sm" | "md";
    icon?: Component;
  }>(),
  { confirmLabel: "Confirmer", size: "xs" },
);

const emit = defineEmits<{ confirm: [] }>();

const open = ref(false);

defineExpose({ reset: () => (open.value = false) });
</script>

<template>
  <BaseButton variant="danger" :size="size" :icon="icon" @click="open = true">{{ label }}</BaseButton>
  <ConfirmDialog
    v-if="open"
    :message="props.message ?? `${props.label} ? Cette action est irréversible.`"
    :confirm-label="props.confirmLabel"
    :busy="props.busy"
    @confirm="emit('confirm')"
    @close="open = false"
  />
</template>
