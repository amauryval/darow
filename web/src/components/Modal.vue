<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";

const props = withDefaults(defineProps<{ title: string; wide?: boolean }>(), { wide: false });
const emit = defineEmits<{ close: [] }>();

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") emit("close");
}

onMounted(() => document.addEventListener("keydown", onKeydown));
onUnmounted(() => document.removeEventListener("keydown", onKeydown));
</script>

<template>
  <Teleport to="body">
    <div class="modal modal-open">
      <div class="modal-box" :class="props.wide ? 'max-w-2xl' : 'max-w-md'">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-slate-900">{{ props.title }}</h2>
          <button class="btn btn-ghost btn-xs btn-circle" @click="emit('close')">✕</button>
        </div>
        <slot />
      </div>
      <div class="modal-backdrop" @click="emit('close')" />
    </div>
  </Teleport>
</template>
