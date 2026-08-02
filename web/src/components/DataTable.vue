<script setup lang="ts" generic="TData">
import type { Row, Table } from "@tanstack/vue-table";
import { FlexRender } from "@tanstack/vue-table";

defineProps<{
  table: Table<TData>;
  emptyMessage?: string;
  rowClass?: (row: Row<TData>) => string;
  loading?: boolean;
}>();

defineEmits<{ rowClick: [row: Row<TData>] }>();

function getCellMetaAlign(meta: unknown): string | undefined {
  if (typeof meta === 'object' && meta !== null) {
    const obj = meta as Record<string, unknown>;
    return obj.align === 'right' ? 'text-right' : undefined;
  }
  return undefined;
}

function getCellMetaClass(meta: unknown): string | undefined {
  if (typeof meta === 'object' && meta !== null) {
    const obj = meta as Record<string, unknown>;
    return typeof obj.class === 'string' ? obj.class : undefined;
  }
  return undefined;
}

// title only for plain scalar values: a rich cell (badge, button, link)
// would otherwise get a redundant/nonsensical tooltip from its coerced text.
function getCellTitle(value: unknown): string | undefined {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean" ? String(value) : undefined;
}

// A row is only a keyboard target when it actually looks clickable — most
// rowClass callers only add "cursor-pointer" for rows that do something on
// click (TableDataPanel's selected-row deselect isn't every row).
function isRowClickable(rowClass: string | undefined): boolean {
  return rowClass?.includes("cursor-pointer") ?? false;
}
</script>

<template>
  <div class="card min-h-0 flex-1 overflow-auto rounded-lg border border-base-300 bg-base-100">
    <table class="table table-zebra table-pin-rows w-max min-w-full text-sm">
      <thead class="text-xs font-medium tracking-wide text-base-content/60 uppercase">
        <tr v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
          <th
            v-for="header in headerGroup.headers"
            :key="header.id"
            class="whitespace-nowrap"
            :class="{ 'cursor-pointer select-none hover:text-base-content': header.column.getCanSort() }"
            :title="header.column.getCanSort() ? 'Maj+clic pour trier sur plusieurs colonnes' : undefined"
            @click="header.column.getToggleSortingHandler()?.($event)"
          >
            <span class="inline-flex items-center gap-1">
              <FlexRender :render="header.column.columnDef.header" :props="header.getContext()" />
              <span v-if="header.column.getCanSort()">
                {{ header.column.getIsSorted() === "asc" ? "↓" : header.column.getIsSorted() === "desc" ? "↑" : "" }}
                <sub v-if="table.getState().sorting.length > 1 && header.column.getIsSorted()">{{ header.column.getSortIndex() + 1 }}</sub>
              </span>
            </span>
          </th>
        </tr>
      </thead>
      <tbody>
        <template v-for="row in table.getRowModel().rows" :key="row.id">
          <!-- The row's own name link (or the parent's action button) is the
               real, focusable control; this keydown is only the keyboard
               equivalent of the row-wide mouse shortcut (§0.20, §8.7). -->
          <tr
            :class="rowClass?.(row)"
            :tabindex="isRowClickable(rowClass?.(row)) ? 0 : undefined"
            @click="$emit('rowClick', row)"
            @keydown.enter="isRowClickable(rowClass?.(row)) && $emit('rowClick', row)"
            @keydown.space.prevent="isRowClickable(rowClass?.(row)) && $emit('rowClick', row)"
          >
            <td
              v-for="cell in row.getVisibleCells()"
              :key="cell.id"
              class="max-w-[280px] truncate"
              :class="[getCellMetaAlign(cell.column.columnDef.meta), getCellMetaClass(cell.column.columnDef.meta)]"
              :title="getCellTitle(cell.getValue())"
            >
              <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
            </td>
          </tr>
          <slot name="row-after" :row="row" />
        </template>
        <tr v-if="!loading && table.getRowModel().rows.length === 0">
          <td :colspan="table.getVisibleLeafColumns().length || 1" class="px-4 py-8 text-center text-sm text-base-content/40">
            {{ emptyMessage ?? "Aucun résultat." }}
          </td>
        </tr>
      </tbody>
    </table>
    <slot name="footer" />
  </div>
</template>
