<script setup lang="ts">
import BaseInput from "../ui/BaseInput.vue";

// A cell of the grid while the edit mode is on. Every editable column is one:
// a session makes the whole table writable, not one row of it, so there is no
// per-row button to arm and none to close.
//
// `dirty` marks the cell a save would write — amber, the colour a pending
// write carries everywhere, the same one the map draws a moved geometry in.
// The column names the field: a grid header is not an accessible name for the
// input under it (§8.7).
defineProps<{ modelValue?: string; column: string; dirty?: boolean }>();
defineEmits<{ "update:modelValue": [value: string] }>();
</script>

<template>
  <BaseInput
    :model-value="modelValue"
    size="xs"
    :aria-label="column"
    class="min-w-32"
    :class="dirty ? '[&_input]:border-warning [&_input]:bg-warning/10' : ''"
    @update:model-value="$emit('update:modelValue', $event)"
  />
</template>
