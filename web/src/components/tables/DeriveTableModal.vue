<script setup lang="ts">
import { computed, ref, toRef } from "vue";
import type { ColumnSchema } from "../../client/types";
import { useDeriveTable } from "../../composables/useDeriveTable";
import { useSavedFilters } from "../../composables/useSavedFilters";
import { useToast } from "../../composables/useToast";
import Modal from "../Modal.vue";
import DeriveFromTableForm from "./DeriveFromTableForm.vue";
import DeriveSqlForm from "./DeriveSqlForm.vue";
import BaseButton from "../ui/BaseButton.vue";
import BaseInput from "../ui/BaseInput.vue";
import Typography from "../ui/Typography.vue";

// Two modes, deliberately separated: "Depuis cette table" derives from the
// table in hand, offering its columns and reusing the filter editor for its
// conditions; "SQL" is the free query, which reads every registered table.
// They used to be one SQL box pre-filled with this table's name, which stated
// a scope the endpoint never had.
const props = defineProps<{
  tableId: string;
  physicalName: string;
  columns: ColumnSchema[];
  geometryColumn?: string;
}>();
const emit = defineEmits<{ close: []; derived: [{ tableId: string }] }>();

const toast = useToast();
const {
  isLoading,
  error,
  selectedColumns,
  computedColumns,
  conditions,
  conditionsBool,
  incompleteComputed,
  toggleColumn,
  addComputed,
  updateComputed,
  removeComputed,
  submitSpec,
  submitQuery,
} = useDeriveTable(toRef(props, "tableId"), toRef(props, "columns"));
const { filters: savedFilters } = useSavedFilters(toRef(props, "tableId"));

const mode = ref<"table" | "sql">("table");
const name = ref("");
const query = ref(`SELECT * FROM "${props.physicalName}"`);

const canSubmit = computed(() => {
  if (name.value.trim() === "" || isLoading.value) return false;
  if (mode.value === "sql") return query.value.trim() !== "";
  return !incompleteComputed.value;
});

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3).
const dirty = computed(
  () =>
    name.value.trim() !== "" ||
    selectedColumns.value.length > 0 ||
    computedColumns.value.length > 0 ||
    conditions.value.length > 0,
);

async function onSubmit() {
  if (!canSubmit.value) return;
  const result =
    mode.value === "sql"
      ? await submitQuery(query.value.trim(), name.value.trim())
      : await submitSpec(name.value.trim());
  // A null result means the call failed and `error` already carries the
  // message — it stays in front of the user rather than going out as a
  // toast (§8.4).
  if (result === null) return;

  emit("derived", result);
  emit("close");
  toast.success("Table dérivée créée.");
}
</script>

<template>
  <Modal title="Dériver une table" wide :dirty="dirty" @close="emit('close')" @submit="onSubmit">
    <div role="tablist" class="tabs tabs-box mb-4">
      <button
        role="tab"
        class="tab"
        :class="{ 'tab-active': mode === 'table' }"
        :aria-selected="mode === 'table'"
        @click="mode = 'table'"
      >
        Depuis cette table
      </button>
      <button
        role="tab"
        class="tab"
        :class="{ 'tab-active': mode === 'sql' }"
        :aria-selected="mode === 'sql'"
        @click="mode = 'sql'"
      >
        SQL
      </button>
    </div>

    <DeriveFromTableForm
      v-if="mode === 'table'"
      :table-id="tableId"
      :columns="columns"
      :geometry-column="geometryColumn"
      :selected-columns="selectedColumns"
      :computed-columns="computedColumns"
      :conditions="conditions"
      :conditions-bool="conditionsBool"
      :saved-filters="savedFilters"
      @toggle-column="toggleColumn"
      @add-computed="addComputed"
      @update-computed="updateComputed"
      @remove-computed="removeComputed"
      @update:conditions="conditions = $event"
      @update:conditions-bool="conditionsBool = $event"
    />
    <DeriveSqlForm v-else v-model="query" />

    <div class="mt-4 border-t border-base-300 pt-4">
      <BaseInput v-model="name" size="sm" label="Nom de la nouvelle table" placeholder="Nom de la nouvelle table" />
      <Typography variant="caption" class="mt-1 block">
        La table dérivée est un instantané figé : elle n'est jamais resynchronisée avec sa source.
      </Typography>
    </div>

    <p v-if="error" class="mt-2 text-xs text-error" role="alert">{{ error }}</p>

    <div class="mt-4 flex justify-end gap-2">
      <BaseButton variant="secondary" @click="emit('close')">Annuler</BaseButton>
      <BaseButton
        variant="primary"
        :busy="isLoading"
        :disabled="!canSubmit"
        :title="incompleteComputed ? 'Complétez chaque colonne calculée.' : undefined"
        @click="onSubmit"
      >
        {{ isLoading ? "Création…" : "Créer" }}
      </BaseButton>
    </div>
  </Modal>
</template>
