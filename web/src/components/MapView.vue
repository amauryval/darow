<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import "maplibre-gl/dist/maplibre-gl.css";
import type { FilterCondition } from "../client/types";
import { useMapLibre } from "../composables/useMapLibre";
import { useMvtLayer } from "../composables/useMvtLayer";
import { useWfsLayer } from "../composables/useWfsLayer";

const props = defineProps<{
  tableId: string;
  bbox?: string;
  conditions: FilterCondition[];
  selectedPkeys: string[];
}>();

// Whoever owns selection (useFeatureSelection, in the parent) applies the
// toggle — this component only reports the click, it never mutates
// selection state itself.
const emit = defineEmits<{ toggleSelection: [pkey: string] }>();

// MVT and WFS are two independent rendering modes for the same table/
// filter, toggled manually by the user — not tied to whether a filter is
// active.
const wfsEnabled = ref(false);

const container = ref<HTMLDivElement | null>(null);
const tableId = toRef(props, "tableId");
const bbox = toRef(props, "bbox");
const conditions = computed(() => props.conditions);
const selectedPkeys = computed(() => props.selectedPkeys);

const { map } = useMapLibre(container, bbox, () => {
  if (wfsEnabled.value) {
    wfs.activate();
  } else {
    mvt.activate();
  }
});

const mvt = useMvtLayer(map, tableId, bbox, conditions, {
  selectedPkeys,
  toggle: (pkey) => emit("toggleSelection", pkey),
});
const wfs = useWfsLayer(map, tableId, conditions);

// MVT and WFS are mutually exclusive on the map — switching modes tears
// down the previous mode's source/layers entirely rather than layering
// both, so there is only ever one rendering of the table at a time.
watch(wfsEnabled, (enabled) => {
  if (!map.value) return;
  if (enabled) {
    mvt.deactivate();
    wfs.activate();
  } else {
    wfs.deactivate();
    mvt.activate();
  }
});
</script>

<template>
  <div class="relative h-full w-full">
    <label
      class="absolute top-2 left-2 z-10 flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 shadow"
    >
      <input v-model="wfsEnabled" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" />
      Rendu WFS
    </label>
    <div ref="container" class="h-full w-full rounded-lg" />
  </div>
</template>
