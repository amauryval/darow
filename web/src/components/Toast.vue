<script setup lang="ts">
import { useToast } from "../composables/useToast";

const { toasts, dismiss } = useToast();
</script>

<template>
  <div class="toast toast-end toast-bottom z-50">
    <TransitionGroup name="fade">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="alert max-w-sm text-sm shadow-lg"
        :class="t.kind === 'success' ? 'alert-success' : 'alert-error'"
      >
        <span class="flex-1">{{ t.message }}</span>
        <button class="btn btn-ghost btn-xs" @click="dismiss(t.id)">✕</button>
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
