<script setup lang="ts">
import BaseTextarea from "../ui/BaseTextarea.vue";
import Typography from "../ui/Typography.vue";

// The free-SQL mode. Its query runs against every registered table, not only
// the one whose page this modal was opened from — which is why the caption
// says so: the pre-filled "SELECT * FROM <cette table>" otherwise reads as a
// scope the endpoint does not have.
defineProps<{ modelValue: string }>();
defineEmits<{ "update:modelValue": [value: string] }>();
</script>

<template>
  <div>
    <Typography variant="caption" class="mb-2 block">
      Requête <code class="rounded bg-base-200 px-1">SELECT</code> exécutée sur l'ensemble des tables enregistrées, pas
      seulement celle-ci. Le résultat est matérialisé dans une nouvelle table.
    </Typography>
    <BaseTextarea
      :model-value="modelValue"
      :rows="5"
      aria-label="Requête SQL"
      @update:model-value="$emit('update:modelValue', $event)"
    />
  </div>
</template>
