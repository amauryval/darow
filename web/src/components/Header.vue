<script setup lang="ts">
import { useRoute } from "vue-router";
import { LogIn } from "@lucide/vue";

const route = useRoute();
</script>

<template>
  <header class="sticky top-0 z-20 shrink-0 border-b border-slate-200 bg-white/90 px-6 py-2 backdrop-blur">
    <div class="flex items-center justify-between gap-4">
      <span class="text-sm font-semibold text-slate-900">{{ route.meta.title }}</span>
      <!-- No auth backend yet — placeholder for when the API gains one -->
      <button class="btn btn-sm btn-outline btn-primary gap-1.5" disabled title="Authentification bientôt disponible">
        <LogIn :size="14" />
        Se connecter
      </button>
    </div>
  </header>
</template>
