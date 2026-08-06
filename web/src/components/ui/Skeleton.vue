<script setup lang="ts">
// First-load placeholder for a server-fed list/table (§8.1): grey bars at
// roughly the shape of the rows about to appear, so the screen isn't blank
// while the first page is in flight.
withDefaults(defineProps<{ rows?: number; columns?: number }>(), { rows: 6, columns: 4 })
</script>

<template>
    <div class="divide-y divide-base-200 rounded-lg border border-base-300 bg-base-100">
        <div v-for="r in rows" :key="r" class="flex items-center gap-4 px-4 py-3">
            <div v-for="c in columns" :key="c" class="skeleton h-3 flex-1 first:max-w-[40%]" />
        </div>
    </div>
</template>
