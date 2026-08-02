<script setup lang="ts">
import { ref } from "vue";
import { FileUp, UploadCloud } from "@lucide/vue";
import Modal from "../Modal.vue";
import BaseButton from "../ui/BaseButton.vue";
import Typography from "../ui/Typography.vue";

// Stated before the attempt, not only in an error response (§8.5). Static
// text matching the server's default config.yaml (sources.maxSizeBytes /
// sources.allowedExtensions) — there is no endpoint exposing the running
// deployment's actual values, so a custom config drifts from this hint.
const ACCEPTED_EXTENSIONS = [".geojson", ".shp", ".csv", ".gpkg", ".parquet"];
const MAX_UPLOAD_LABEL = "100 Mo";

defineProps<{ uploading?: boolean }>();
const emit = defineEmits<{ close: []; upload: [file: File] }>();

const dragging = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

function onFileSelected(event: Event) {
  const file = event.target instanceof HTMLInputElement ? event.target.files?.[0] : undefined;
  // Cleared before emitting: picking the same file twice in a row emits no
  // change event otherwise.
  if (fileInput.value) fileInput.value.value = "";
  if (file) emit("upload", file);
}

function onDrop(event: DragEvent) {
  dragging.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (file) emit("upload", file);
}
</script>

<template>
  <Modal title="Importer une source" @close="emit('close')">
    <div
      class="flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 text-center transition-colors"
      :class="dragging ? 'border-primary bg-primary/10' : 'border-base-300 bg-base-200/50'"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <UploadCloud :size="36" class="text-base-content/40" />
      <Typography variant="body-sm">Glissez un fichier ici</Typography>
      <Typography variant="caption">ou</Typography>
      <input ref="fileInput" type="file" class="hidden" :accept="ACCEPTED_EXTENSIONS.join(',')" @change="onFileSelected" />
      <BaseButton variant="primary" size="sm" :icon="FileUp" :busy="uploading" :disabled="uploading" @click="fileInput?.click()">
        {{ uploading ? "Envoi…" : "Parcourir" }}
      </BaseButton>
      <Typography variant="caption">{{ ACCEPTED_EXTENSIONS.join(", ") }} — {{ MAX_UPLOAD_LABEL }} maximum</Typography>
    </div>
  </Modal>
</template>
