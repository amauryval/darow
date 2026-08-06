<script setup lang="ts">
import Modal from "./Modal.vue";
import BaseButton from "./BaseButton.vue";
import Typography from "./Typography.vue";

// The confirmation of a destructive action, worded once for the whole app —
// always this box, never a native confirm() and never an inline pair of
// buttons (§8.3). `ConfirmButton` is it with its own trigger; a caller whose
// trigger is something else (a menu entry) opens this one directly.
defineProps<{
  // Names what is about to disappear and any cascade effect.
  message: string;
  confirmLabel?: string;
  busy?: boolean;
}>();
const emit = defineEmits<{ confirm: []; close: [] }>();
</script>

<template>
  <Modal title="Confirmer la suppression" @close="emit('close')" @submit="emit('confirm')">
    <Typography variant="body-sm">{{ message }}</Typography>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="emit('close')">Annuler</BaseButton>
      <BaseButton variant="danger" :busy="busy" @click="emit('confirm')">{{ confirmLabel ?? "Confirmer" }}</BaseButton>
    </div>
  </Modal>
</template>
