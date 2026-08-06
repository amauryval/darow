<script setup lang="ts">
import { toRef } from 'vue'
import { Eye, EyeOff } from '@lucide/vue'
import { useColumnDistinctValues } from '../../composables/useColumnDistinctValues'
import Modal from '../ui/Modal.vue'
import Badge from '../ui/Badge.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseCheckbox from '../ui/BaseCheckbox.vue'

const props = defineProps<{ tableId: string; rowColumns: string[]; visibleColumns: string[] }>()
const emit = defineEmits<{ 'update:visibleColumns': [columns: string[]]; close: [] }>()

function isVisible(col: string) {
    return props.visibleColumns.includes(col)
}

function toggleVisible(col: string) {
    emit(
        'update:visibleColumns',
        isVisible(col)
            ? props.visibleColumns.filter((c) => c !== col)
            : [...props.visibleColumns, col]
    )
}

const distinct = useColumnDistinctValues(toRef(props, 'tableId'))
</script>

<template>
    <Modal title="Colonnes" wide @close="emit('close')">
        <div class="flex max-h-96 flex-col gap-1 overflow-y-auto pr-1">
            <div v-for="col in rowColumns" :key="col" class="rounded-md border border-base-300">
                <div class="flex items-center justify-between px-3 py-2">
                    <label class="flex items-center gap-2 text-sm">
                        <BaseCheckbox
                            :model-value="isVisible(col)"
                            @update:model-value="toggleVisible(col)"
                        />
                        {{ col }}
                    </label>
                    <BaseButton
                        variant="ghost"
                        size="xs"
                        :icon="distinct.expandedColumn.value === col ? EyeOff : Eye"
                        @click="distinct.toggle(col)"
                    >
                        {{
                            distinct.expandedColumn.value === col
                                ? 'Masquer les valeurs'
                                : 'Voir les valeurs'
                        }}
                    </BaseButton>
                </div>
                <div
                    v-if="distinct.expandedColumn.value === col"
                    class="border-t border-base-200 bg-base-200 px-3 py-2"
                >
                    <p v-if="distinct.loading.value" class="text-xs text-base-content/40">
                        Chargement…
                    </p>
                    <p v-else-if="distinct.error.value" class="text-xs text-error">
                        {{ distinct.error.value }}
                    </p>
                    <div v-else class="flex flex-wrap gap-1.5">
                        <Badge v-for="(v, i) in distinct.values.value" :key="i" tone="neutral">{{
                            v === null ? 'NULL' : v
                        }}</Badge>
                        <span
                            v-if="distinct.values.value.length === 0"
                            class="text-xs text-base-content/40"
                            >Aucune valeur.</span
                        >
                    </div>
                </div>
            </div>
        </div>
    </Modal>
</template>
