<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import Modal from "./Modal.vue";
import BaseButton from "./ui/BaseButton.vue";
import BaseInput from "./ui/BaseInput.vue";

// On a geo table the geometry is not typed here: the modal collects the
// attributes and the map takes over for the shape (useGeometryEditing).
const props = defineProps<{ editableColumns: string[]; isGeo?: boolean }>();
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
        <span class="mb-1 block text-xs font-medium text-base-content/50">{{ col }}</span>
        <BaseInput v-model="newRowValues[col]" />
      </label>
    </div>
    <p v-if="isGeo" class="mt-3 text-xs text-base-content/60">
      La géométrie se dessine ensuite sur la carte : la ligne sera créée une fois la forme enregistrée.
    </p>
    <p v-if="error" class="mt-3 text-xs text-error">{{ error }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="onClose">Annuler</BaseButton>
      <BaseButton variant="primary" :busy="isLoading" :disabled="isLoading" @click="confirmAdd">
        {{ isGeo ? "Dessiner la géométrie" : isLoading ? "Ajout…" : "Ajouter" }}
      </BaseButton>
    </div>
  </Modal>
</template>
