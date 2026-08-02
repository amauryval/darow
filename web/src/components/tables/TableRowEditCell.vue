<script setup lang="ts">
defineProps<{ modelValue?: string; column?: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();

function getInputValue(event: Event): string {
  const target = event.target;
  return target instanceof HTMLInputElement ? target.value : "";
}
</script>

<template>
  <input
    :value="modelValue"
    type="text"
    class="w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
    @input="emit('update:modelValue', getInputValue($event))"
  />
</template>
