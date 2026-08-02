<script setup lang="ts">
import { ref } from "vue";
import type { Component } from "vue";
import Modal from "./Modal.vue";
import BaseButton from "./ui/BaseButton.vue";
import Typography from "./ui/Typography.vue";

// A destructive action gated by the shared daisyUI `Modal` (never a native
// confirm()) rather than an inline pair — consistent across a single rare
// action (delete a table) and a frequent one (delete a row) alike (§8.3).
withDefaults(
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

function onConfirm() {
  emit("confirm");
}

defineExpose({ reset: () => (open.value = false) });
</script>

<template>
  <BaseButton variant="danger" :size="size" :icon="icon" @click="open = true">{{ label }}</BaseButton>
  <Modal v-if="open" title="Confirmer la suppression" @close="open = false" @submit="onConfirm">
    <Typography variant="body-sm">{{ message ?? `${label} ? Cette action est irréversible.` }}</Typography>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="open = false">Annuler</BaseButton>
      <BaseButton variant="danger" :busy="busy" @click="onConfirm">{{ confirmLabel }}</BaseButton>
    </div>
  </Modal>
</template>
