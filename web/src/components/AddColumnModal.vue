<script setup lang="ts">
import { ref } from "vue";
import { errorMessage } from "../client/http";
import { addColumn } from "../client/tables";
import Modal from "./Modal.vue";
import { useToast } from "../composables/useToast";
import BaseButton from "./ui/BaseButton.vue";

const props = defineProps<{ tableId: string }>();
const emit = defineEmits<{ close: []; added: [] }>();

const toast = useToast();
const name = ref("");
const type = ref("VARCHAR");
const error = ref<string | null>(null);
const isLoading = ref(false);

function openModal() {
  name.value = "";
  type.value = "VARCHAR";
  error.value = null;
}

async function confirmAdd() {
  if (!name.value.trim()) return;
  isLoading.value = true;
  error.value = null;
  try {
    await addColumn(props.tableId, name.value.trim(), type.value.trim());
    emit("added");
    emit("close");
    toast.success("Colonne ajoutée.");
  } catch (e) {
    error.value = errorMessage(e, "Impossible d'ajouter la colonne");
  } finally {
    isLoading.value = false;
  }
}

function onClose() {
  emit("close");
}

defineExpose({ openModal });
</script>

<template>
  <Modal title="Ajouter une colonne" :dirty="!!name.trim()" @close="onClose" @submit="confirmAdd">
    <label class="mb-3 block">
      <span class="mb-1 block text-xs font-medium text-slate-500">Nom</span>
      <input
        v-model="name"
        type="text"
        class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </label>
    <label class="block">
      <span class="mb-1 block text-xs font-medium text-slate-500">Type DuckDB</span>
      <input
        v-model="type"
        type="text"
        placeholder="VARCHAR, INTEGER, DOUBLE…"
        class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 font-mono text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </label>
    <p v-if="error" class="mt-3 text-xs text-red-600">{{ error }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="onClose">Annuler</BaseButton>
      <BaseButton variant="primary" :busy="isLoading" :disabled="isLoading || !name.trim()" @click="confirmAdd">
        {{ isLoading ? "Ajout…" : "Ajouter" }}
      </BaseButton>
    </div>
  </Modal>
</template>
