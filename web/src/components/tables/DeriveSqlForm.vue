<script setup lang="ts">
import BaseTextarea from '../ui/BaseTextarea.vue'
import Typography from '../ui/Typography.vue'

// The free-SQL mode, opened from the catalogue (DeriveSqlModal) rather than
// one table's page — the caption states its scope (every registered table)
// since nothing on screen otherwise names it.
defineProps<{ modelValue: string }>()
defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
    <div>
        <Typography variant="caption" class="mb-2 block">
            Requête <code class="rounded bg-base-200 px-1">SELECT</code> exécutée sur l'ensemble des
            tables enregistrées. Le résultat est matérialisé dans une nouvelle table.
        </Typography>
        <BaseTextarea
            :model-value="modelValue"
            :rows="5"
            aria-label="Requête SQL"
            @update:model-value="$emit('update:modelValue', $event)"
        />
    </div>
</template>
