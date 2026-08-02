<script setup lang="ts">
import { ref } from "vue";

// A destructive action rendered as a single button that, on first click,
// swaps itself for an inline Confirm/Cancel pair instead of a native
// confirm() dialog — kept local to the button so it can sit inline in a
// table row without a modal.
const props = withDefaults(
  defineProps<{
    label: string;
    confirmLabel?: string;
    busy?: boolean;
  }>(),
  { confirmLabel: "Confirmer" },
);

const emit = defineEmits<{ confirm: [] }>();

const confirming = ref(false);

function onClick() {
  if (confirming.value) {
    emit("confirm");
  } else {
    confirming.value = true;
  }
}

defineExpose({ reset: () => (confirming.value = false) });
</script>

<template>
  <div v-if="confirming" class="inline-flex items-center gap-1.5">
    <span class="text-xs text-slate-500">Sûr ?</span>
    <button class="btn btn-error btn-xs" :disabled="busy" @click="onClick">
      {{ busy ? "…" : props.confirmLabel }}
    </button>
    <button class="btn btn-ghost btn-xs" @click="confirming = false">Annuler</button>
  </div>
  <button v-else class="btn btn-outline btn-error btn-xs" @click="onClick">
    {{ props.label }}
  </button>
</template>
