<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { RouterLink, RouterView } from "vue-router";
import { FolderOpen, LayoutGrid, MapPinned } from "@lucide/vue";
import { readyz } from "./client/ops";
import Header from "./components/Header.vue";
import Toast from "./components/Toast.vue";

type Status = "checking" | "ok" | "down";

const status = ref<Status>("checking");
let interval: ReturnType<typeof setInterval> | undefined;

async function checkStatus() {
  try {
    await readyz();
    status.value = "ok";
  } catch {
    status.value = "down";
  }
}

onMounted(() => {
  checkStatus();
  interval = setInterval(checkStatus, 20_000);
});
onUnmounted(() => clearInterval(interval));

const statusLabel: Record<Status, string> = {
  checking: "Vérification…",
  ok: "Opérationnel",
  down: "Indisponible",
};

const statusDotClass: Record<Status, string> = {
  checking: "bg-slate-300",
  ok: "bg-emerald-500",
  down: "bg-red-500",
};
</script>

<template>
  <div class="flex h-screen bg-slate-50 text-slate-900">
    <aside class="flex w-60 shrink-0 flex-col border-r border-slate-200 bg-white">
      <div class="flex items-center gap-2.5 px-5 py-5">
        <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-600 text-white">
          <MapPinned :size="16" />
        </span>
        <div class="leading-tight">
          <p class="text-sm font-semibold tracking-tight text-slate-900">Darow — get your Ducks in A ROW</p>
          <p class="text-[11px] text-slate-400">Backed by DuckDB · Console d'administration</p>
        </div>
      </div>

      <nav class="flex flex-col gap-0.5 px-3">
        <RouterLink
          to="/tables"
          class="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-900"
          active-class="!bg-indigo-50 !text-indigo-700"
        >
          <LayoutGrid :size="16" class="shrink-0" />
          Tables
        </RouterLink>
        <RouterLink
          to="/sources"
          class="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-900"
          active-class="!bg-indigo-50 !text-indigo-700"
        >
          <FolderOpen :size="16" class="shrink-0" />
          Sources
        </RouterLink>
      </nav>

      <div class="mt-auto flex items-center gap-2 border-t border-slate-100 px-5 py-3.5">
        <span class="relative flex h-2 w-2">
          <span
            v-if="status === 'ok'"
            class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60"
            :class="statusDotClass[status]"
          />
          <span class="relative inline-flex h-2 w-2 rounded-full" :class="statusDotClass[status]" />
        </span>
        <span class="text-xs font-medium text-slate-500">{{ statusLabel[status] }}</span>
      </div>
    </aside>
    <main class="min-w-0 flex-1 overflow-y-auto">
      <Header />
      <RouterView />
    </main>
    <Toast />
  </div>
</template>
