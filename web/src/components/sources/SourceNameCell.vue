<script setup lang="ts">
import { computed } from "vue";
import { File, Layers, Map as MapIcon } from "@lucide/vue";
import Badge from "../ui/Badge.vue";

const props = defineProps<{
  name: string;
  // A layer of a multi-layer file, rendered indented under its parent row.
  isLayer?: boolean;
  layerCount?: number;
  ext?: string;
}>();

const icon = computed(() => (props.isLayer ? MapIcon : props.layerCount !== undefined ? Layers : File));
</script>

<template>
  <div class="flex items-center gap-2" :class="isLayer ? 'pl-6' : ''">
    <component :is="icon" :size="16" class="shrink-0 text-base-content/40" aria-hidden="true" />
    <span class="truncate" :class="isLayer ? '' : 'font-medium'">{{ name }}</span>
    <Badge v-if="ext && !isLayer" tone="neutral">{{ ext.replace(".", "").toUpperCase() }}</Badge>
  </div>
</template>
