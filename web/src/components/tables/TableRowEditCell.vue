<script setup lang="ts">
import BaseInput from "../ui/BaseInput.vue";

defineProps<{ modelValue?: string; column?: string }>();
defineEmits<{ 'update:modelValue': [value: string] }>();
</script>

<template>
  <BaseInput :model-value="modelValue" size="sm" @update:model-value="$emit('update:modelValue', $event)" />
</template>
