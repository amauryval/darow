<script setup lang="ts">
import { computed, toRef, watch } from "vue";
import { useColumnValues } from "../../composables/useColumnValues";
import { isTextType } from "../../utils/columnTypes";
import BaseAutocomplete from "../ui/BaseAutocomplete.vue";
import BaseInput from "../ui/BaseInput.vue";

// One filter condition's value field. It owns its own suggestion state, so
// two conditions on two columns never share a list.
const props = defineProps<{ tableId: string; column: string; type: string; modelValue: string }>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const suggestible = computed(() => isTextType(props.type));
const values = useColumnValues(toRef(props, "tableId"), toRef(props, "column"));

// The list belongs to the column it was loaded for: switching columns must
// drop it rather than suggest another column's values.
watch(() => [props.tableId, props.column], values.reset);

// NULL has no text form to type, and the distinct page keeps it: it would
// otherwise show up as an empty, unselectable row.
const options = computed(() => values.values.value.filter((v) => v !== null).map(String));
</script>

<template>
  <BaseAutocomplete
    v-if="suggestible"
    :model-value="modelValue"
    :options="options"
    :loading="values.loading.value"
    size="sm"
    placeholder="Valeur"
    aria-label="Valeur"
    @update:model-value="emit('update:modelValue', $event)"
    @search="values.search"
  />
  <BaseInput
    v-else
    :model-value="modelValue"
    size="sm"
    placeholder="Valeur"
    aria-label="Valeur"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>
