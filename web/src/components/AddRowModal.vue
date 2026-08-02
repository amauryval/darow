<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import Modal from "./Modal.vue";
import BaseButton from "./ui/BaseButton.vue";

const props = defineProps<{ editableColumns: string[]; geometryColumn?: string }>();
const emit = defineEmits<{ close: []; add: [values: Record<string, string>] }>();

const newRowValues = reactive<Record<string, string>>({});
const isDirty = computed(() => Object.values(newRowValues).some((v) => v.trim() !== ""));
const isLoading = ref(false);
const error = ref<string | null>(null);

function openModal() {
  for (const col of props.editableColumns) newRowValues[col] = "";
  error.value = null;
}

async function confirmAdd() {
  isLoading.value = true;
  try {
    emit("add", newRowValues);
    emit("close");
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
  <Modal title="Ajouter une ligne" :dirty="isDirty" @close="onClose" @submit="confirmAdd">
    <div class="flex max-h-96 flex-col gap-3 overflow-y-auto pr-1">
      <label v-for="col in editableColumns" :key="col" class="block">
        <span class="mb-1 block text-xs font-medium text-slate-500">
          {{ col }} <span v-if="col === geometryColumn">(WKT)</span>
        </span>
        <input
          v-model="newRowValues[col]"
          type="text"
          :placeholder="col === geometryColumn ? 'POINT(2.35 48.85)' : ''"
          class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
        />
      </label>
    </div>
    <p v-if="error" class="mt-3 text-xs text-red-600">{{ error }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="onClose">Annuler</BaseButton>
      <BaseButton variant="primary" :busy="isLoading" :disabled="isLoading" @click="confirmAdd">
        {{ isLoading ? "Ajout…" : "Ajouter" }}
      </BaseButton>
    </div>
  </Modal>
</template>
