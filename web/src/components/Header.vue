<script setup lang="ts">
import { useRoute } from "vue-router";
import { LogIn } from "@lucide/vue";

const route = useRoute();
</script>

<template>
  <header class="navbar sticky top-0 z-20 min-h-0 border-b border-slate-200 bg-white/90 px-6 py-2.5 backdrop-blur">
    <div class="flex-1">
      <span class="text-sm font-semibold text-slate-900">{{ route.meta.title }}</span>
    </div>
    <div class="flex-none">
      <!-- No auth backend yet — this is a placeholder for when the API
      gains one; disabled rather than removed so the layout it needs
      (navbar end slot) is already in place. -->
      <button class="btn btn-sm btn-outline btn-primary gap-1.5" disabled title="Authentification bientôt disponible">
        <LogIn :size="14" />
        Se connecter
      </button>
    </div>
  </header>
</template>
