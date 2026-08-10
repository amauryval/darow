<script setup lang="ts">
import { ref } from 'vue'
import BaseTextarea from '../ui/BaseTextarea.vue'
import DocLink from '../ui/DocLink.vue'
import SqlSchemaReference from './SqlSchemaReference.vue'
import Typography from '../ui/Typography.vue'

const DUCKDB_SELECT_URL = 'https://duckdb.org/docs/sql/statements/select'

// The free-SQL mode, opened from the catalogue (DeriveSqlModal) rather than
// one table's page — the caption states its scope (every registered table)
// since nothing on screen otherwise names it. SqlSchemaReference sits beside
// the textarea rather than above or below it: a query is written with the
// schema in view, not memorised from a panel scrolled past.
defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const textarea = ref<InstanceType<typeof BaseTextarea> | null>(null)

function insert(text: string) {
    textarea.value?.insertAtCursor(text)
}
</script>

<template>
    <div>
        <Typography variant="caption" class="mb-2 block">
            Requête <code class="rounded bg-base-200 px-1">SELECT</code> exécutée sur l'ensemble des
            tables enregistrées. Le résultat est matérialisé dans une nouvelle table.
            <DocLink :href="DUCKDB_SELECT_URL">Syntaxe SQL DuckDB</DocLink>
        </Typography>
        <div class="flex gap-3">
            <BaseTextarea
                ref="textarea"
                :model-value="modelValue"
                :rows="10"
                class="flex-1"
                aria-label="Requête SQL"
                @update:model-value="emit('update:modelValue', $event)"
            />
            <div class="h-[15rem] w-52 shrink-0">
                <SqlSchemaReference @insert="insert" />
            </div>
        </div>
    </div>
</template>
