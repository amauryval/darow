<script setup lang="ts">
import { computed } from "vue";
import Badge from "../Badge.vue";
import BaseButton from "../ui/BaseButton.vue";

// What the session holds and the two buttons that decide its fate. The counts
// stand with them rather than at the other end of the bar: they are what makes
// *Enregistrer* mean something, and they are stated apart because they are
// three different writes and they fail differently (§8.2).
//
// The pair lives in one place per table: inside the map's tools on a geo
// table (they are teleported above both panels, so they stay reachable when
// the map is the hidden half), in the edit bar itself otherwise.
const props = defineProps<{
  // Geometries reshaped, shapes drawn, rows whose cells were retyped.
  changedCount: number;
  createdCount: number;
  rowCount: number;
  saving?: boolean;
}>();

const emit = defineEmits<{ save: []; cancel: [] }>();

const dirtyCount = computed(() => props.changedCount + props.createdCount + props.rowCount);
</script>

<template>
  <div class="flex items-center gap-2">
    <Badge v-if="rowCount > 0" tone="warning">{{ rowCount }} ligne{{ rowCount > 1 ? "s" : "" }}</Badge>
    <Badge v-if="changedCount > 0" tone="warning">
      {{ changedCount }} géométrie{{ changedCount > 1 ? "s" : "" }}
    </Badge>
    <Badge v-if="createdCount > 0" tone="primary">
      {{ createdCount }} nouvelle{{ createdCount > 1 ? "s" : "" }}
    </Badge>

    <BaseButton variant="ghost" size="sm" :disabled="saving" @click="emit('cancel')">Annuler</BaseButton>
    <BaseButton
      variant="primary"
      size="sm"
      :busy="saving"
      :disabled="saving || dirtyCount === 0"
      :title="dirtyCount === 0 ? 'Rien n’a été modifié' : undefined"
      @click="emit('save')"
    >
      {{ saving ? "Enregistrement…" : "Enregistrer" }}
    </BaseButton>
  </div>
</template>
