<script setup lang="ts">
import { computed } from 'vue'
import { X } from '@lucide/vue'
import type { ColumnSchema } from '../../client/types'
import type { DraftNode } from '../../utils/filterTree'
import type { AssignmentDraft } from '../../utils/transformAssignment'
import FilterNodeEditor from '../filters/FilterNodeEditor.vue'
import AssignmentEditor from './AssignmentEditor.vue'
import BaseButton from '../ui/BaseButton.vue'
import Typography from '../ui/Typography.vue'

// One arm of the transform's CASE WHEN: the rows it applies to and the
// value they receive. A case with no condition renders indistinguishable
// from the default assignment, so the server refuses it (§5.11) — the
// caller blocks submission on the same rule (useColumnTransform.incomplete).
const props = defineProps<{
    conditions: DraftNode[]
    conditionsBool: 'AND' | 'OR'
    assignment: AssignmentDraft
    tableId: string
    targetColumn: ColumnSchema
    columns: ColumnSchema[]
}>()
const emit = defineEmits<{
    'update:conditions': [nodes: DraftNode[]]
    'update:conditionsBool': [bool: 'AND' | 'OR']
    'update:assignment': [draft: AssignmentDraft]
    remove: []
}>()

// Mirrors useColumnTransform.incomplete's per-case check (an inactive node
// is dropped before the server ever sees it, so it does not count either):
// stated here, next to the "+ Condition" button the user actually needs,
// rather than only on the modal's disabled submit button.
const needsCondition = computed(() => !props.conditions.some((n) => n.active))
</script>

<template>
    <div class="rounded-md border border-base-300 p-3">
        <div class="mb-2 flex items-center justify-between">
            <Typography variant="eyebrow">Où</Typography>
            <BaseButton
                variant="ghost"
                size="xs"
                square
                :icon="X"
                icon-only
                aria-label="Supprimer ce cas"
                @click="emit('remove')"
            />
        </div>
        <FilterNodeEditor
            :table-id="tableId"
            :columns="columns"
            :nodes="conditions"
            :bool="conditionsBool"
            @update:nodes="emit('update:conditions', $event)"
            @update:bool="emit('update:conditionsBool', $event)"
        />
        <p v-if="needsCondition" class="mt-1 text-xs text-warning">
            Ajoutez au moins une condition à ce cas, ou supprimez-le et utilisez « Sinon » seul.
        </p>
        <Typography variant="eyebrow" class="mt-3 block">Assigner</Typography>
        <AssignmentEditor
            class="mt-1"
            :table-id="tableId"
            :target-column="targetColumn"
            :columns="columns"
            :model-value="assignment"
            @update:model-value="emit('update:assignment', $event)"
        />
    </div>
</template>
