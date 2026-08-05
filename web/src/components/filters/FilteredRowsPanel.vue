<script setup lang="ts">
import { computed, h } from "vue";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import type { Row } from "../../client/types";
import { cellValue } from "../../utils/rows";
import DataTable from "../DataTable.vue";
import TableRowActions from "../tables/TableRowActions.vue";
import BaseInput from "../ui/BaseInput.vue";
import ErrorState from "../ui/ErrorState.vue";
import Skeleton from "../ui/Skeleton.vue";
import Typography from "../ui/Typography.vue";

// The filtered view is flat and read-only, capped at `limit` rows: it is a
// server-side query result, not the paginated, editable grid. rows is null
// while the first result set is still in flight or after a failure.
const props = defineProps<{
  visibleColumns: string[];
  rows: Row[] | null;
  loading: boolean;
  error: string;
  limit: number;
  isGeo?: boolean;
  rowClass?: (row: { original: Row }) => string;
}>();
const emit = defineEmits<{
  "update:limit": [limit: number];
  retry: [];
  "row-click": [row: { original: Row }];
  geometry: [pkey: string];
}>();

const columnHelper = createColumnHelper<Row>();
// The filtered grid is read-only, but reading a row's geometry is a read:
// it stays reachable here too, or the feature would disappear as soon as a
// filter is applied.
const columnDefs = computed(() => {
  const defs = props.visibleColumns.map((col) =>
    columnHelper.accessor((row) => cellValue(row, col), { id: col, header: col }),
  );
  if (!props.isGeo) return defs;
  // Same place as in the paginated grid: at the start of the row and pinned
  // there, so filtering a table does not move the control that reads a
  // geometry to the far end of a grid that scrolls sideways.
  return [
    columnHelper.display({
      id: "actions",
      header: "",
      meta: { pinned: "left-0", class: "w-px px-2 whitespace-nowrap" },
      cell: (info) => h(TableRowActions, { isGeo: true, onGeometry: () => emit("geometry", info.row.original.pkey) }),
    }),
    ...defs,
  ];
});

const table = useVueTable({
  get data() {
    return props.rows ?? [];
  },
  get columns() {
    return columnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
});
</script>

<template>
  <div class="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-2">
    <Typography variant="body-sm">{{ rows?.length ?? 0 }} ligne(s) filtrée(s)</Typography>
    <label class="flex items-center gap-2">
      <Typography variant="caption">Limite</Typography>
      <BaseInput
        :model-value="limit"
        type="number"
        inputmode="numeric"
        size="xs"
        class="w-24"
        aria-label="Nombre maximum de lignes filtrées"
        @update:model-value="emit('update:limit', Number($event) || 1)"
      />
    </label>
  </div>

  <Skeleton v-if="loading && rows === null" :rows="8" />
  <ErrorState v-else-if="error" :message="error" @retry="emit('retry')" />
  <DataTable
    v-else
    :table="table"
    :loading="loading"
    :row-class="rowClass"
    empty-message="Pas de données pour ce filtre."
    @row-click="emit('row-click', $event)"
  />
</template>
