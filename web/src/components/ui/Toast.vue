<script setup lang="ts">
import { useToast } from "../../composables/useToast";
import BaseButton from "./BaseButton.vue";

const { toasts, dismiss } = useToast();
</script>

<template>
  <div class="toast toast-end toast-bottom z-50">
    <TransitionGroup name="fade">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="alert w-80 max-w-md items-start text-sm shadow-lg"
        :class="t.kind === 'success' ? 'alert-success' : 'alert-error'"
        :role="t.kind === 'error' ? 'alert' : 'status'"
        :aria-live="t.kind === 'error' ? 'assertive' : 'polite'"
      >
        <span class="flex-1 [overflow-wrap:anywhere] select-text whitespace-pre-line">{{ t.message }}</span>
        <BaseButton variant="ghost" size="xs" class="shrink-0" aria-label="Fermer la notification" @click="dismiss(t.id)">✕</BaseButton>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
