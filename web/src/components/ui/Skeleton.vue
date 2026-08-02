<script setup lang="ts">
// First-load placeholder for a server-fed list/table (§8.1): grey bars at
// roughly the shape of the rows about to appear, so the screen isn't blank
// while the first page is in flight.
withDefaults(defineProps<{ rows?: number; columns?: number }>(), { rows: 6, columns: 4 });
</script>

<template>
  <div class="motion-reduce:animate-none animate-pulse divide-y divide-slate-100 rounded-lg border border-slate-200 bg-white">
    <div v-for="r in rows" :key="r" class="flex items-center gap-4 px-4 py-3">
      <div v-for="c in columns" :key="c" class="h-3 flex-1 rounded bg-slate-200 first:max-w-[40%]" />
    </div>
  </div>
</template>
