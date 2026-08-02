<script setup lang="ts">
import { ref } from "vue";
import { RouterLink } from "vue-router";
import { Pencil } from "@lucide/vue";
import { renameTable } from "../../client/tables";
import { errorMessage } from "../../client/http";
import type { Metadata } from "../../client/types";
import { useToast } from "../../composables/useToast";
import BaseButton from "../ui/BaseButton.vue";
import BaseInput from "../ui/BaseInput.vue";

const props = defineProps<{ tableId: string; tableName: string }>();
const emit = defineEmits<{ renamed: [table: Metadata] }>();

const toast = useToast();
const editing = ref(false);
const nameInput = ref("");
const renaming = ref(false);

function startRename() {
  nameInput.value = props.tableName;
  editing.value = true;
}

async function confirmRename() {
  if (!nameInput.value.trim() || nameInput.value.trim() === props.tableName) {
    editing.value = false;
    return;
  }
  renaming.value = true;
  try {
    const table = await renameTable(props.tableId, nameInput.value.trim());
    editing.value = false;
    emit("renamed", table);
    toast.success("Table renommée.");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de renommer la table"));
  } finally {
    renaming.value = false;
  }
}
</script>

<template>
  <div v-if="editing" class="flex items-center gap-1.5" @click.stop>
    <BaseInput v-model="nameInput" size="sm" @keyup.enter="confirmRename" @keyup.esc="editing = false" />
    <BaseButton size="sm" variant="primary" :busy="renaming" :disabled="renaming || !nameInput.trim()" @click="confirmRename">
      Enregistrer
    </BaseButton>
    <BaseButton size="sm" variant="secondary" @click="editing = false">Annuler</BaseButton>
  </div>
  <div v-else class="group flex items-center gap-1.5">
    <RouterLink :to="`/tables/${tableId}`" class="link link-primary font-medium no-underline" @click.stop>{{ tableName }}</RouterLink>
    <BaseButton
      variant="ghost"
      size="xs"
      square
      :icon="Pencil"
      icon-only
      aria-label="Renommer la table"
      class="opacity-0 group-hover:opacity-100"
      @click.stop="startRename"
    />
  </div>
</template>
