<script setup lang="ts">
import { RouterLink, RouterView } from "vue-router";
</script>

<template>
  <div class="flex h-screen bg-slate-50 text-slate-900">
    <aside class="flex w-56 shrink-0 flex-col border-r border-slate-200 bg-white">
      <div class="px-5 py-4">
        <span class="text-sm font-semibold tracking-wide text-slate-900">GeoAPI</span>
      </div>
      <nav class="flex flex-col gap-0.5 px-3">
        <RouterLink
          to="/tables"
          class="rounded-md px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900"
          active-class="bg-slate-100 text-slate-900"
        >
          Tables
        </RouterLink>
        <RouterLink
          to="/sources"
          class="rounded-md px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900"
          active-class="bg-slate-100 text-slate-900"
        >
          Sources
        </RouterLink>
      </nav>
    </aside>
    <main class="min-w-0 flex-1 overflow-y-auto">
      <RouterView />
    </main>
  </div>
</template>
