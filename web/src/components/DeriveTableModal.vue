<script setup lang="ts">
import { ref } from "vue";
import { errorMessage } from "../client/http";
import { deriveTable } from "../client/tables";
import Modal from "./Modal.vue";
import { useToast } from "../composables/useToast";
import BaseButton from "./ui/BaseButton.vue";

const props = defineProps<{ tableId: string; physicalName: string }>();
const emit = defineEmits<{ close: []; derived: [{ tableId: string }] }>();

const toast = useToast();
const query = ref("");
const name = ref("");
const error = ref<string | null>(null);
const isLoading = ref(false);

function openModal() {
  query.value = `SELECT * FROM "${props.physicalName}"`;
  name.value = "";
  error.value = null;
}

async function confirmDerive() {
  if (!query.value.trim() || !name.value.trim()) return;
  isLoading.value = true;
  error.value = null;
  try {
    const result = await deriveTable(props.tableId, query.value.trim(), name.value.trim());
    emit("derived", result);
    emit("close");
    toast.success("Table dérivée créée.");
  } catch (e) {
    error.value = errorMessage(e, "Impossible de dériver la table");
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
  <Modal title="Dériver une table" wide :dirty="!!name.trim()" @close="onClose" @submit="confirmDerive">
    <p class="mb-3 text-xs text-slate-500">
      Requête <code class="rounded bg-slate-100 px-1 py-0.5">SELECT</code> exécutée sur les tables existantes ; le résultat est
      matérialisé dans une nouvelle table.
    </p>
    <textarea
      v-model="query"
      rows="4"
      spellcheck="false"
      class="w-full resize-y rounded-md border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
    />
    <input
      v-model="name"
      type="text"
      placeholder="Nom de la nouvelle table"
      class="mt-3 w-full rounded-md border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
    />
    <p v-if="error" class="mt-2 text-xs text-red-600">{{ error }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="onClose">Annuler</BaseButton>
      <BaseButton
        variant="primary"
        :busy="isLoading"
        :disabled="isLoading || !query.trim() || !name.trim()"
        @click="confirmDerive"
      >
        {{ isLoading ? "Création…" : "Créer" }}
      </BaseButton>
    </div>
  </Modal>
</template>
