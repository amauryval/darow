<script setup lang="ts">
import { ref } from "vue";
import { errorMessage } from "../../client/http";
import { useToast } from "../../composables/useToast";
import Modal from "../ui/Modal.vue";
import BaseButton from "../ui/BaseButton.vue";
import BaseInput from "../ui/BaseInput.vue";

// The write is the caller's useTableMetadata instance (onAdd): it already
// owns the table's schema and reloads it after adding a column, so a second
// call here would be a second owner of the same state (§0.5, §0.12).
const props = defineProps<{ onAdd: (name: string, type: string) => Promise<void> }>();
const emit = defineEmits<{ close: [] }>();

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
    await props.onAdd(name.value.trim(), type.value.trim());
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
    <BaseInput v-model="name" label="Nom" class="mb-3" />
    <BaseInput v-model="type" label="Type" placeholder="VARCHAR, INTEGER, DOUBLE…" class="font-mono" />
    <p v-if="error" class="mt-3 text-xs text-error">{{ error }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="onClose">Annuler</BaseButton>
      <BaseButton variant="primary" :busy="isLoading" :disabled="isLoading || !name.trim()" @click="confirmAdd">
        {{ isLoading ? "Ajout…" : "Ajouter" }}
      </BaseButton>
    </div>
  </Modal>
</template>
