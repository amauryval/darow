<script setup lang="ts">
import { Plus, X } from '@lucide/vue'
import type { ColumnSchema } from '../../client/types'
import { columnType } from '../../utils/columnTypes'
import {
    createLeaf,
    nextNodeId,
    patchGroupBool,
    patchLeaf,
    removeNode as removeNodeUtil,
    takesValue,
    toggleNot,
    updateChildren as updateChildrenUtil,
    type DraftLeaf,
    type DraftNode
} from '../../utils/filterTree'
import FilterValueInput from './FilterValueInput.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseSelect from '../ui/BaseSelect.vue'
import Typography from '../ui/Typography.vue'

// IS NULL / IS NOT NULL take no value: they are the only way to match a NULL,
// since SQL's "col = NULL" is never true.
const OPERATORS = ['=', '!=', '>', '<', '>=', '<=', 'LIKE', 'NOT LIKE', 'IS NULL', 'IS NOT NULL']

// The boolean connector is a left-hand column, not a sentence above the list:
// "Où", then the one editable ET/OU, then that word repeated as plain text.
// The predicate reads straight down, a nested group indenting under it.
const props = defineProps<{
    tableId: string
    columns: ColumnSchema[]
    nodes: DraftNode[]
    bool: 'AND' | 'OR'
}>()
const emit = defineEmits<{
    'update:nodes': [nodes: DraftNode[]]
    'update:bool': [bool: 'AND' | 'OR']
}>()

function updateLeaf(id: number, patch: Partial<Pick<DraftLeaf, 'column' | 'operator' | 'value'>>) {
    emit('update:nodes', patchLeaf(props.nodes, id, patch))
}

function updateGroupBool(id: number, bool: 'AND' | 'OR') {
    emit('update:nodes', patchGroupBool(props.nodes, id, bool))
}

function toggleNodeNot(id: number) {
    emit('update:nodes', toggleNot(props.nodes, id))
}

function removeNode(id: number) {
    emit('update:nodes', removeNodeUtil(props.nodes, id))
}

function updateChildren(groupId: number, children: DraftNode[]) {
    emit('update:nodes', updateChildrenUtil(props.nodes, groupId, children))
}

const firstColumn = () => props.columns[0]?.name ?? ''

function addLeaf() {
    emit('update:nodes', [...props.nodes, createLeaf(firstColumn())])
}

function addGroup() {
    emit('update:nodes', [
        ...props.nodes,
        {
            id: nextNodeId(),
            kind: 'group',
            bool: 'AND',
            children: [createLeaf(firstColumn())],
            not: false,
            active: true
        }
    ])
}

// The type is part of the option's label: it decides what a value has to look
// like, and which conditions make sense at all (a LIKE on an integer matches
// nothing).
const columnOptions = () =>
    props.columns.map((col) => ({ value: col.name, label: `${col.name} — ${col.type}` }))
const operatorOptions = OPERATORS.map((op) => ({ value: op, label: op }))
const boolOptions = [
    { value: 'AND', label: 'ET' },
    { value: 'OR', label: 'OU' }
]
const boolLabel = () => (props.bool === 'AND' ? 'ET' : 'OU')
</script>

<template>
    <div class="flex flex-col gap-1">
        <div v-for="(node, index) in nodes" :key="node.id" class="flex items-start gap-2">
            <div class="w-14 shrink-0 pt-1.5">
                <Typography v-if="index === 0" variant="caption">Où</Typography>
                <BaseSelect
                    v-else-if="index === 1"
                    :model-value="bool"
                    size="xs"
                    class="w-full"
                    aria-label="Connecteur entre les conditions"
                    :options="boolOptions"
                    @update:model-value="emit('update:bool', $event === 'OR' ? 'OR' : 'AND')"
                />
                <Typography v-else variant="caption">{{ boolLabel() }}</Typography>
            </div>

            <div class="min-w-0 flex-1">
                <div v-if="node.kind === 'leaf'" class="flex items-center gap-1.5">
                    <BaseSelect
                        :model-value="node.column"
                        size="sm"
                        class="min-w-0 flex-1"
                        :options="columnOptions()"
                        aria-label="Colonne"
                        @update:model-value="updateLeaf(node.id, { column: $event })"
                    />
                    <BaseSelect
                        :model-value="node.operator"
                        size="sm"
                        class="w-32 shrink-0"
                        :options="operatorOptions"
                        aria-label="Opérateur"
                        @update:model-value="updateLeaf(node.id, { operator: $event })"
                    />
                    <FilterValueInput
                        v-if="takesValue(node.operator)"
                        :table-id="tableId"
                        :column="node.column"
                        :type="columnType(columns, node.column)"
                        :model-value="node.value"
                        class="min-w-0 flex-1"
                        @update:model-value="updateLeaf(node.id, { value: $event })"
                    />
                    <div v-else class="min-w-0 flex-1"></div>
                </div>

                <div v-else class="rounded-md bg-base-200 px-2 py-1.5">
                    <FilterNodeEditor
                        :table-id="tableId"
                        :columns="columns"
                        :nodes="node.children"
                        :bool="node.bool"
                        @update:nodes="updateChildren(node.id, $event)"
                        @update:bool="updateGroupBool(node.id, $event)"
                    />
                </div>
            </div>

            <div class="flex shrink-0 items-center gap-0.5 pt-1">
                <BaseButton
                    size="xs"
                    :variant="node.not ? 'primary' : 'ghost'"
                    :aria-pressed="node.not"
                    :title="
                        node.kind === 'group'
                            ? 'Inverser tout le groupe'
                            : 'Inverser cette condition'
                    "
                    @click="toggleNodeNot(node.id)"
                >
                    NON
                </BaseButton>
                <BaseButton
                    variant="ghost"
                    size="xs"
                    square
                    class="text-base-content/40 hover:text-error"
                    :aria-label="
                        node.kind === 'group' ? 'Supprimer le groupe' : 'Supprimer la condition'
                    "
                    @click="removeNode(node.id)"
                >
                    <X :size="14" aria-hidden="true" />
                </BaseButton>
            </div>
        </div>

        <div class="flex gap-1 pl-16">
            <BaseButton variant="ghost" size="xs" :icon="Plus" @click="addLeaf"
                >Condition</BaseButton
            >
            <BaseButton variant="ghost" size="xs" :icon="Plus" @click="addGroup">Groupe</BaseButton>
        </div>
    </div>
</template>
