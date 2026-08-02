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
      <BaseButton size="sm" variant="primary" :busy="isSaving" :disabled="isSaving" @click="$emit('save')">
        {{ isSaving ? "…" : "Enregistrer" }}
      </BaseButton>
      <BaseButton size="sm" variant="secondary" @click="$emit('cancel')">Annuler</BaseButton>
    </template>
    <template v-else>
      <button class="text-xs font-medium text-indigo-600 hover:text-indigo-700" @click="$emit('edit')">Modifier</button>
      <ConfirmButton label="Supprimer" :busy="isDeletingPkey === deletingPkey" @confirm="$emit('delete')" />
    </template>
  </div>
</template>
