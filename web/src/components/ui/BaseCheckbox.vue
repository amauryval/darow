<script setup lang="ts">
withDefaults(defineProps<{ modelValue?: boolean; size?: "xs" | "sm" | "md" }>(), { size: "xs" });

const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();

const sizes = {
  xs: "checkbox-xs",
  sm: "checkbox-sm",
  md: "",
};

function onChange(event: Event) {
  if (event.target instanceof HTMLInputElement) emit("update:modelValue", event.target.checked);
}
</script>

<template>
  <input type="checkbox" class="checkbox" :class="sizes[size]" :checked="modelValue" @change="onChange" />
</template>
