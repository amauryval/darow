<script setup lang="ts">
import { Pencil, Shapes } from "@lucide/vue";
import BaseButton from "../ui/BaseButton.vue";
import ConfirmButton from "../ConfirmButton.vue";

// editable is what edit mode turns on: reading a row's geometry is a read,
// available whether or not the grid is armed for editing.
withDefaults(
  defineProps<{
    editable?: boolean;
    isGeo?: boolean;
    isEditing?: boolean;
    isSaving?: boolean;
    isDeletingPkey?: string;
    deletingPkey?: string;
  }>(),
  { editable: false, isGeo: false, isEditing: false, isSaving: false },
);

defineEmits<{
  save: [];
  cancel: [];
  edit: [];
  delete: [];
  geometry: [];
  editGeometry: [];
}>();
</script>

<template>
  <div class="flex justify-end gap-2">
    <template v-if="editable && isEditing">
      <!-- Same size as the read-mode pair: a larger edit-mode button resized
           the actions column and shifted the whole row on every edit. -->
      <BaseButton size="xs" variant="primary" :busy="isSaving" :disabled="isSaving" @click="$emit('save')">
        {{ isSaving ? "…" : "Enregistrer" }}
      </BaseButton>
      <BaseButton size="xs" variant="secondary" @click="$emit('cancel')">Annuler</BaseButton>
    </template>
    <template v-else>
      <BaseButton
        v-if="isGeo"
        size="xs"
        variant="ghost"
        :icon="Shapes"
        icon-only
        title="Voir la géométrie (WKT, GeoJSON)"
        @click="$emit('geometry')"
      >
        Géométrie
      </BaseButton>
      <template v-if="editable">
        <BaseButton
          v-if="isGeo"
          size="xs"
          variant="ghost"
          :icon="Pencil"
          icon-only
          title="Modifier la géométrie sur la carte"
          @click="$emit('editGeometry')"
        >
          Modifier la géométrie
        </BaseButton>
        <BaseButton variant="secondary" size="xs" @click="$emit('edit')">Modifier</BaseButton>
        <ConfirmButton
          label="Supprimer"
          message="Supprimer définitivement cette ligne ?"
          :busy="isDeletingPkey === deletingPkey"
          @confirm="$emit('delete')"
        />
      </template>
    </template>
  </div>
</template>
