<script setup lang="ts" generic="TData">
import type { ColumnMeta, Row, Table } from "@tanstack/vue-table";
import { FlexRender } from "@tanstack/vue-table";

defineProps<{
  table: Table<TData>;
  emptyMessage?: string;
  rowClass?: (row: Row<TData>) => string;
  loading?: boolean;
}>();

const emit = defineEmits<{ rowClick: [row: Row<TData>] }>();

// A click that landed on a control inside the row belongs to that control,
// never to the row: the row-wide shortcut would otherwise also toggle the
// selection of the very row the user asked to edit, duplicate or delete.
function onRowClick(row: Row<TData>, event: MouseEvent) {
  const target = event.target;
  if (target instanceof Element && target.closest("button, a, input, select, textarea, label")) return;
  emit("rowClick", row);
}

function getCellMetaAlign(meta: ColumnMeta<TData, unknown> | undefined): string | undefined {
  return meta?.align === "right" ? "text-right" : undefined;
}

function getCellMetaClass(meta: ColumnMeta<TData, unknown> | undefined): string | undefined {
  return meta?.class;
}

// A column that stays put while the grid scrolls sideways. The caller says
// where its left edge sits (a Tailwind offset, `left-0` for the first), since
// only it knows the widths of the ones before. The cell takes the row's own
// background rather than a colour of its own (`bg-inherit`), so a pinned cell
// stripes and highlights exactly like the row it belongs to — which is why
// every row here carries an **opaque** background of its own instead of
// daisyUI's zebra: a translucent one would let the scrolled columns show
// through the pinned cells.
// `level` is the stacking order: a pinned header cell has to stay above the
// pinned body cells that scroll under it, and both above the ordinary ones.
function getPinnedClass(meta: ColumnMeta<TData, unknown> | undefined, level: 1 | 2): string | undefined {
  return meta?.pinned ? `sticky ${meta.pinned} bg-inherit ${level === 2 ? "z-[2]" : "z-[1]"}` : undefined;
}

// title only for plain scalar values: a rich cell (badge, button, link)
// would otherwise get a redundant/nonsensical tooltip from its coerced text.
function getCellTitle(value: unknown): string | undefined {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean" ? String(value) : undefined;
}

// A row is only a keyboard target when it actually looks clickable — a
// rowClass only carries "cursor-pointer" on rows whose click does something
// (a table with no geometry has no selection to toggle).
function isRowClickable(rowClass: string | undefined): boolean {
  return rowClass?.includes("cursor-pointer") ?? false;
}
</script>

<template>
  <div class="card min-h-0 flex-1 overflow-auto rounded-lg border border-base-300 bg-base-100">
    <table class="table table-pin-rows w-max min-w-full text-sm">
      <thead class="text-xs font-medium tracking-wide text-base-content/60 uppercase">
        <tr v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
          <th
            v-for="header in headerGroup.headers"
            :key="header.id"
            class="whitespace-nowrap"
            :class="[
              getCellMetaAlign(header.column.columnDef.meta),
              getPinnedClass(header.column.columnDef.meta, 2),
              { 'cursor-pointer select-none hover:text-base-content': header.column.getCanSort() },
            ]"
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
            class="bg-base-100 even:bg-base-200"
            :class="rowClass?.(row)"
            :tabindex="isRowClickable(rowClass?.(row)) ? 0 : undefined"
            @click="onRowClick(row, $event)"
            @keydown.enter="isRowClickable(rowClass?.(row)) && emit('rowClick', row)"
            @keydown.space.prevent="isRowClickable(rowClass?.(row)) && emit('rowClick', row)"
          >
            <td
              v-for="cell in row.getVisibleCells()"
              :key="cell.id"
              class="max-w-[280px] truncate"
              :class="[
                getCellMetaAlign(cell.column.columnDef.meta),
                getCellMetaClass(cell.column.columnDef.meta),
                getPinnedClass(cell.column.columnDef.meta, 1),
              ]"
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
