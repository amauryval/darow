<script setup lang="ts">
const props = defineProps<{ offset: number; limit: number; total: number }>();
const emit = defineEmits<{ prev: []; next: [] }>();
</script>

<template>
  <div class="flex items-center justify-between text-sm text-slate-500">
    <span>{{ total === 0 ? 0 : props.offset + 1 }}–{{ Math.min(props.offset + props.limit, props.total) }} sur {{ props.total }}</span>
    <div class="join">
      <button class="btn btn-sm join-item" :disabled="props.offset === 0" @click="emit('prev')">Précédent</button>
      <button class="btn btn-sm join-item" :disabled="props.offset + props.limit >= props.total" @click="emit('next')">
        Suivant
      </button>
    </div>
  </div>
</template>
