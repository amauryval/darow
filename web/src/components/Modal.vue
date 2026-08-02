<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useFocusTrap } from "../composables/useFocusTrap";

// dirty: a modified form must not be lost by a stray click (§8.3). While
// dirty, a backdrop click is a no-op and Escape asks for confirmation
// instead of closing straight away.
const props = withDefaults(defineProps<{ title: string; wide?: boolean; dirty?: boolean }>(), {
  wide: false,
  dirty: false,
});
const emit = defineEmits<{ close: []; submit: [] }>();

const box = ref<HTMLElement | null>(null);
useFocusTrap(box);

const confirmingClose = ref(false);

function onBackdropClick() {
  if (props.dirty) return;
  emit("close");
}

function requestClose() {
  if (props.dirty && !confirmingClose.value) {
    confirmingClose.value = true;
    return;
  }
  emit("close");
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    requestClose();
    return;
  }
  // Enter submits the modal's form (§8.5) — except inside a textarea, where
  // it means a newline, and on a button, which already handles its own Enter.
  if (e.key === "Enter" && !(e.target instanceof HTMLTextAreaElement) && !(e.target instanceof HTMLButtonElement)) {
    emit("submit");
  }
}

onMounted(() => document.addEventListener("keydown", onKeydown));
onUnmounted(() => document.removeEventListener("keydown", onKeydown));
</script>

<template>
  <Teleport to="body">
    <div class="modal modal-open">
      <div ref="box" class="modal-box" :class="props.wide ? 'max-w-2xl' : 'max-w-md'">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-slate-900">{{ props.title }}</h2>
          <button class="btn btn-ghost btn-xs btn-circle" aria-label="Fermer" @click="requestClose">✕</button>
        </div>
        <slot />
        <div v-if="confirmingClose" class="mt-3 flex items-center justify-between rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">
          <span>Fermer sans enregistrer les modifications ?</span>
          <div class="flex gap-2">
            <button class="font-medium hover:underline" @click="emit('close')">Fermer</button>
            <button class="font-medium hover:underline" @click="confirmingClose = false">Continuer</button>
          </div>
        </div>
      </div>
      <div class="modal-backdrop" @click="onBackdropClick" />
    </div>
  </Teleport>
</template>
