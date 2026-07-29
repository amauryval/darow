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
</script>

<template>
  <div class="min-h-0 flex-1 overflow-auto rounded-lg border border-slate-200 bg-white">
    <table class="w-max min-w-full text-sm">
      <thead class="sticky top-0 z-10 border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
        <tr v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
          <th
            v-for="header in headerGroup.headers"
            :key="header.id"
            class="whitespace-nowrap px-4 py-2.5"
            :class="{ 'cursor-pointer select-none hover:text-slate-700': header.column.getCanSort() }"
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
      <tbody class="divide-y divide-slate-100">
        <template v-for="row in table.getRowModel().rows" :key="row.id">
          <tr :class="rowClass?.(row)" @click="$emit('rowClick', row)">
            <td
              v-for="cell in row.getVisibleCells()"
              :key="cell.id"
              class="whitespace-nowrap px-4 py-2.5 text-slate-700"
              :class="[
                (cell.column.columnDef.meta as { align?: string; class?: string } | undefined)?.align === 'right' ? 'text-right' : '',
                (cell.column.columnDef.meta as { align?: string; class?: string } | undefined)?.class,
              ]"
            >
              <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
            </td>
          </tr>
          <slot name="row-after" :row="row" />
        </template>
        <tr v-if="!loading && table.getRowModel().rows.length === 0">
          <td :colspan="table.getVisibleLeafColumns().length || 1" class="px-4 py-8 text-center text-sm text-slate-400">
            {{ emptyMessage ?? "Aucun résultat." }}
          </td>
        </tr>
      </tbody>
    </table>
    <slot name="footer" />
  </div>
</template>
