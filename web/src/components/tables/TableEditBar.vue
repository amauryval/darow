<script setup lang="ts">
import { Plus } from "@lucide/vue";
import BaseButton from "../ui/BaseButton.vue";
import Divider from "../ui/Divider.vue";
import Typography from "../ui/Typography.vue";

// The bar an edit session raises above **both** panels: editing is one mode
// of the table, and what it offers is of two kinds — what applies to the rows
// and what applies to the geometries. Only what applies is shown: a table
// without geometry has the first group alone.
//
// The map's own tools are rendered into `toolsId` by `MapView` (a Teleport):
// they need the OpenLayers session that lives inside the map component, which
// must not be pulled into the initial bundle by this bar (§8.10).
defineProps<{ isGeo: boolean; toolsId: string }>();
const emit = defineEmits<{ addRow: [] }>();
</script>

<template>
  <div class="flex shrink-0 flex-wrap  gap-x-3 gap-y-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2">
    <div class="flex items-center gap-2 py-0.5">
      <Typography variant="eyebrow">Lignes</Typography>
      <BaseButton variant="primary" size="sm" :icon="Plus" @click="emit('addRow')">Ajouter une ligne</BaseButton>
    </div>

    <template v-if="isGeo">
      <Divider />
      <div class="flex min-w-0 flex-1 gap-2">
        <div :id="toolsId" class="min-w-0 flex-1" />
      </div>
    </template>
  </div>
</template>
