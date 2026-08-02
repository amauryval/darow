<script setup lang="ts">
import BaseButton from "../ui/BaseButton.vue";
import ConfirmButton from "../ConfirmButton.vue";

defineProps<{
  isEditing: boolean;
  isSaving: boolean;
  isDeletingPkey?: string;
  deletingPkey?: string;
}>();

defineEmits<{
  save: [];
  cancel: [];
  edit: [];
  delete: [];
}>();
</script>

<template>
  <div class="flex justify-end gap-2">
    <template v-if="isEditing">
      <!-- Same size as the read-mode pair: a larger edit-mode button resized
           the actions column and shifted the whole row on every edit. -->
      <BaseButton size="xs" variant="primary" :busy="isSaving" :disabled="isSaving" @click="$emit('save')">
        {{ isSaving ? "…" : "Enregistrer" }}
      </BaseButton>
      <BaseButton size="xs" variant="secondary" @click="$emit('cancel')">Annuler</BaseButton>
    </template>
    <template v-else>
      <BaseButton variant="secondary" size="xs" @click="$emit('edit')">Modifier</BaseButton>
      <ConfirmButton
        label="Supprimer"
        message="Supprimer définitivement cette ligne ?"
        :busy="isDeletingPkey === deletingPkey"
        @confirm="$emit('delete')"
      />
    </template>
  </div>
</template>
