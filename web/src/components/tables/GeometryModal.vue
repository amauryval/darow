<script setup lang="ts">
import { computed } from "vue";
import { Copy } from "@lucide/vue";
import { excerpt, formatGeoJSON } from "../../utils/geometry";
import Modal from "../ui/Modal.vue";
import BaseButton from "../ui/BaseButton.vue";
import ErrorState from "../ui/ErrorState.vue";
import Skeleton from "../ui/Skeleton.vue";
import Typography from "../ui/Typography.vue";

// The geometry of a single row, in the two encodings it is read for. It is
// presentational: what is loaded, retried or copied is decided by
// useRowGeometry in the panel above.
const props = defineProps<{
  pkey: string;
  geometry: GeoJSON.Geometry | null;
  wkt: string;
  loading: boolean;
  error: string;
}>();
const emit = defineEmits<{ close: []; retry: []; copy: [text: string, label: string] }>();

const geoJSON = computed(() => formatGeoJSON(props.geometry));
const hasGeometry = computed(() => props.geometry !== null || props.wkt !== "");
</script>

<template>
  <Modal title="Géométrie de la ligne" wide @close="emit('close')">
    <Typography variant="caption">Ligne {{ pkey }} — coordonnées en longitude/latitude (OGC:CRS84).</Typography>

    <Skeleton v-if="loading" :rows="6" />
    <ErrorState v-else-if="error" :message="error" @retry="emit('retry')" />
    <Typography v-else-if="!hasGeometry" variant="body-sm" class="mt-4 block">
      Cette ligne ne porte aucune géométrie.
    </Typography>

    <div v-else class="mt-4 flex flex-col gap-4">
      <section class="flex flex-col gap-2">
        <div class="flex items-center justify-between gap-2">
          <Typography variant="eyebrow">WKT</Typography>
          <BaseButton size="xs" variant="secondary" :icon="Copy" @click="emit('copy', wkt, 'WKT')">Copier</BaseButton>
        </div>
        <pre class="max-h-48 overflow-auto rounded-lg bg-base-200 p-3 text-xs whitespace-pre-wrap">{{ excerpt(wkt) }}</pre>
      </section>

      <section class="flex flex-col gap-2">
        <div class="flex items-center justify-between gap-2">
          <Typography variant="eyebrow">GeoJSON</Typography>
          <BaseButton size="xs" variant="secondary" :icon="Copy" @click="emit('copy', geoJSON, 'GeoJSON')">
            Copier
          </BaseButton>
        </div>
        <pre class="max-h-64 overflow-auto rounded-lg bg-base-200 p-3 text-xs whitespace-pre-wrap">{{
          excerpt(geoJSON)
        }}</pre>
      </section>
    </div>

    <div class="mt-4 flex justify-end">
      <BaseButton variant="secondary" @click="emit('close')">Fermer</BaseButton>
    </div>
  </Modal>
</template>
