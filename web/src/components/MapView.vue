<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import "maplibre-gl/dist/maplibre-gl.css";
import type { FilterCondition } from "../client/types";
import { useMapDraw, type ReferenceFeature } from "../composables/useMapDraw";
import { useMapLibre } from "../composables/useMapLibre";
import { useMvtLayer } from "../composables/useMvtLayer";
import { useWfsLayer } from "../composables/useWfsLayer";
import { isEditableGeometry } from "../utils/geometry";
import type { DrawShape } from "../utils/vertices";
import { Magnet, MapPin, Pentagon, Spline, Undo2, Waypoints } from "@lucide/vue";
import BaseButton from "./ui/BaseButton.vue";
import BaseCheckbox from "./ui/BaseCheckbox.vue";
import Typography from "./ui/Typography.vue";

const props = defineProps<{
  tableId: string;
  bbox?: string;
  conditions: FilterCondition[];
  selectedPkeys: string[];
  // Drawing is on only while the parent is actually entering a geometry:
  // outside that, the map stays a reading surface.
  drawing?: boolean;
  drawingPrompt?: string;
  initialGeometry?: GeoJSON.Geometry | null;
  // The neighbours the editor snaps to and draws as outlines, and that the
  // parent recuts when the topological mode is on.
  neighbours?: ReferenceFeature[];
  // Whether the edit is carried into the neighbours sharing a boundary.
  // Only meaningful while an existing geometry is reshaped, so the toggle
  // only shows there.
  topology?: boolean;
  saving?: boolean;
  // Bumped by the parent after every write: MapLibre keeps its own copy of
  // a tile per URL, so a rendering only shows an edited geometry once the
  // layers are told to ask again.
  dataVersion?: number;
}>();

// Whoever owns selection (useFeatureSelection, in the parent) applies the
// toggle — this component only reports the click, it never mutates
// selection state itself. The drawn geometry leaves the same way: the
// parent decides what it means and writes it.
const emit = defineEmits<{
  toggleSelection: [pkey: string];
  save: [geometry: GeoJSON.Geometry];
  cancel: [];
  "update:topology": [value: boolean];
}>();

// MVT and WFS are two independent rendering modes for the same table/
// filter, toggled manually by the user — not tied to whether a filter is
// active.
const wfsEnabled = ref(false);

const container = ref<HTMLDivElement | null>(null);
const tableId = toRef(props, "tableId");
const bbox = toRef(props, "bbox");
const conditions = computed(() => props.conditions);
const selectedPkeys = computed(() => props.selectedPkeys);
const dataVersion = computed(() => props.dataVersion ?? 0);

const { map } = useMapLibre(container, bbox, () => {
  if (wfsEnabled.value) {
    wfs.activate();
  } else {
    mvt.activate();
  }
  if (props.drawing) startDrawing();
});

const selection = { selectedPkeys, toggle: (pkey: string) => emit("toggleSelection", pkey) };
const mvt = useMvtLayer(map, tableId, bbox, conditions, selection, dataVersion);
const wfs = useWfsLayer(map, tableId, conditions, selection, dataVersion);
const draw = useMapDraw(map);

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

function startDrawing() {
  const geometry = props.initialGeometry ?? null;
  draw.activate(isEditableGeometry(geometry) ? geometry : null);
  draw.setReference(props.neighbours ?? []);
}

// The neighbours land after the session opens (they are loaded from the
// server once the edited geometry is known), so they are pushed in as they
// arrive rather than only at activation.
watch(
  () => props.neighbours,
  (features) => {
    if (props.drawing) draw.setReference(features ?? []);
  },
);

watch(
  () => props.drawing,
  (drawing) => {
    if (drawing) {
      startDrawing();
    } else {
      draw.deactivate();
    }
  },
);

function onSave() {
  const geometry = draw.pendingGeometry.value;
  if (geometry) emit("save", geometry);
}

// The shape picker is a creation affordance: correcting an existing
// geometry keeps whatever type the row already carries.
const shapes: Array<{ value: DrawShape; label: string; icon: typeof Spline }> = [
  { value: "Polygon", label: "Polygone", icon: Pentagon },
  { value: "LineString", label: "Ligne", icon: Spline },
  { value: "Point", label: "Point", icon: MapPin },
];
</script>

<template>
  <div class="relative h-full w-full">
    <label class="card absolute top-2 left-2 z-10 flex flex-row items-center gap-1.5 bg-base-100 px-2.5 py-1.5 text-xs font-medium shadow">
      <BaseCheckbox v-model="wfsEnabled" />
      Rendu WFS
    </label>

    <!-- The drawing bar sits over the map, always visible while a geometry
         is being entered: a drag is not a submission, saving is a click. -->
    <div
      v-if="drawing"
      class="card absolute inset-x-2 bottom-2 z-10 flex flex-row items-center justify-between gap-3 bg-base-100 px-3 py-2 shadow"
    >
      <Typography variant="body-sm">{{ drawingPrompt }}</Typography>
      <div class="flex items-center gap-2">
        <BaseButton
          v-for="s in initialGeometry ? [] : shapes"
          :key="s.value"
          size="sm"
          :variant="draw.shape.value === s.value ? 'primary' : 'secondary'"
          :icon="s.icon"
          :aria-pressed="draw.shape.value === s.value"
          @click="draw.setShape(s.value)"
        >
          {{ s.label }}
        </BaseButton>
        <BaseButton
          v-if="!initialGeometry"
          size="sm"
          variant="secondary"
          :icon="Undo2"
          title="Retirer le dernier point posé"
          @click="draw.undo"
        >
          Annuler le point
        </BaseButton>
        <BaseButton
          size="sm"
          :variant="draw.snapping.value ? 'primary' : 'secondary'"
          :icon="Magnet"
          :aria-pressed="draw.snapping.value"
          title="Aimanter les sommets aux géométries voisines"
          @click="draw.toggleSnapping"
        >
          Aimantation
        </BaseButton>
        <BaseButton
          v-if="initialGeometry"
          size="sm"
          :variant="topology ? 'primary' : 'secondary'"
          :icon="Waypoints"
          :aria-pressed="topology"
          title="Recouper les voisins pour qu'ils suivent la limite modifiée"
          @click="emit('update:topology', !topology)"
        >
          Topologie
        </BaseButton>
        <BaseButton variant="secondary" size="sm" @click="emit('cancel')">Annuler</BaseButton>
        <BaseButton
          variant="primary"
          size="sm"
          :busy="saving"
          :disabled="saving || draw.pendingGeometry.value === null"
          :title="draw.pendingGeometry.value === null ? 'Dessinez d’abord une géométrie' : undefined"
          @click="onSave"
        >
          {{ saving ? "Enregistrement…" : "Enregistrer" }}
        </BaseButton>
      </div>
    </div>

    <div ref="container" class="h-full w-full rounded-lg" />
  </div>
</template>
