<script setup lang="ts">
import BaseButton from "./BaseButton.vue";

// A list-load failure (§8.1): stays on screen with the cause and a retry,
// distinct from a genuinely empty result — never just a toast that leaves
// the screen looking like an empty list.
defineProps<{ message: string }>();
defineEmits<{ retry: [] }>();
</script>

<template>
  <div class="flex flex-1 flex-col items-center justify-center gap-3 rounded-lg border border-base-300 bg-base-100 px-4 py-10">
    <div role="alert" class="alert alert-error alert-soft max-w-md">
      <span class="text-sm">{{ message }}</span>
    </div>
    <BaseButton variant="secondary" size="sm" @click="$emit('retry')">Réessayer</BaseButton>
  </div>
</template>
