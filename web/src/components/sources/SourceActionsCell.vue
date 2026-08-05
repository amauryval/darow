<script setup lang="ts">
import { computed } from "vue";
import { RouterLink } from "vue-router";
import { ArrowRight, Trash2 } from "@lucide/vue";
import type { SourceRow } from "../../utils/sourceRows";
import ConfirmButton from "../ConfirmButton.vue";
import BaseButton from "../ui/BaseButton.vue";
import BaseInput from "../ui/BaseInput.vue";

const props = defineProps<{
  row: SourceRow;
  // Set while this very row is the one whose EPSG is being asked for; the
  // key comparison is the view's, so the cell stays presentational.
  editingEpsg?: boolean;
  epsg: string;
  // Whether the source has a geometry to project at all. A file of plain
  // columns is never asked for an EPSG: it has nothing to reproject, and the
  // server takes it with no srid (§8.2).
  epsgApplies?: boolean;
  detecting?: boolean;
  loadingRow?: boolean;
  deleting?: boolean;
}>();

const emit = defineEmits<{
  startLoad: [];
  cancelLoad: [];
  confirmLoad: [];
  remove: [];
  "update:epsg": [value: string];
}>();

// The server takes an SRID, a positive integer — a leading zero or a negative
// value is rejected, so the confirm button stays disabled until the input can
// be one. A source with no geometry has nothing to state: it is confirmed as
// it is, and the detection is what says which of the two this is.
const epsgValid = computed(() => /^[1-9]\d*$/.test(props.epsg.trim()));
const confirmable = computed(() => !props.detecting && (props.epsgApplies === false || epsgValid.value));

const deleteMessage = computed(() =>
  props.row.layerCount !== undefined
    ? `Supprimer « ${props.row.source.name} » ? Toutes ses couches chargées seront également supprimées.`
    : props.row.loaded
      ? `Supprimer « ${props.row.source.name} » ? La table chargée associée sera également supprimée.`
      : `Supprimer « ${props.row.source.name} » ?`,
);
</script>

<template>
  <div class="flex items-center justify-end gap-2">
    <template v-if="editingEpsg">
      <BaseInput
        v-if="detecting || epsgApplies !== false"
        :model-value="epsg"
        inputmode="numeric"
        size="xs"
        class="w-24"
        aria-label="Code EPSG"
        :placeholder="detecting ? 'Détection…' : 'Code EPSG'"
        :disabled="detecting"
        @update:model-value="emit('update:epsg', $event)"
      />
      <BaseButton
        variant="primary"
        size="xs"
        :busy="loadingRow"
        :disabled="!confirmable || loadingRow"
        @click="emit('confirmLoad')"
      >
        {{ loadingRow ? "Chargement…" : "Confirmer" }}
      </BaseButton>
      <BaseButton variant="ghost" size="xs" @click="emit('cancelLoad')">Annuler</BaseButton>
    </template>

    <!-- `custom` so the navigation lands on a real button rather than a
         <button> nested in an <a>, which is invalid markup. -->
    <RouterLink v-else-if="row.loaded && row.tableId" v-slot="{ navigate }" :to="`/tables/${row.tableId}`" custom>
      <BaseButton variant="secondary" size="xs" :icon="ArrowRight" @click="navigate">Voir la table</BaseButton>
    </RouterLink>

    <template v-else>
      <BaseButton v-if="row.layerCount === undefined" variant="secondary" size="xs" @click="emit('startLoad')">Charger</BaseButton>
      <ConfirmButton
        v-if="!row.layerName"
        :label="row.layerCount !== undefined ? 'Supprimer le fichier' : 'Supprimer'"
        :icon="Trash2"
        :message="deleteMessage"
        :busy="deleting"
        @confirm="emit('remove')"
      />
    </template>
  </div>
</template>
