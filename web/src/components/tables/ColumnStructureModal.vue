<script setup lang="ts">
import { Check, Pencil, Trash2, X } from '@lucide/vue'
import { useColumnStructure } from '../../composables/useColumnStructure'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import ConfirmDialog from '../ui/ConfirmDialog.vue'
import FormError from '../ui/FormError.vue'
import Typography from '../ui/Typography.vue'

// Renaming and dropping a column, on the columns a caller may touch — the
// pkey and the geometry column are never in `columns`. The writes are the
// caller's useTableMetadata instance: it owns the schema and reloads it,
// as for AddColumnModal (§0.5, §0.12).
const props = defineProps<{
    columns: string[]
    onRename: (name: string, newName: string) => Promise<void>
    onDrop: (name: string) => Promise<void>
}>()
const emit = defineEmits<{ close: [] }>()

const {
    editing,
    draftName,
    confirmingDrop,
    busyColumn,
    error,
    startRename,
    confirmRename,
    confirmDrop
} = useColumnStructure(props.onRename, props.onDrop)
</script>

<template>
    <Modal title="Structure des colonnes" @close="emit('close')">
        <Typography v-if="!columns.length" variant="body-sm">
            Cette table n'a aucune colonne modifiable.
        </Typography>

        <div v-else class="flex max-h-96 flex-col gap-1 overflow-y-auto pr-1">
            <div
                v-for="col in columns"
                :key="col"
                class="flex items-center gap-2 rounded-md border border-base-300 px-3 py-2"
            >
                <template v-if="editing === col">
                    <BaseInput
                        v-model="draftName"
                        size="sm"
                        class="flex-1"
                        :aria-label="`Nouveau nom de la colonne ${col}`"
                        @keyup.enter="confirmRename(col)"
                        @keyup.esc="editing = null"
                    />
                    <BaseButton
                        icon-only
                        :icon="Check"
                        size="xs"
                        variant="primary"
                        :busy="busyColumn === col"
                        :disabled="busyColumn === col || !draftName.trim()"
                        aria-label="Enregistrer le nom"
                        @click="confirmRename(col)"
                    />
                    <BaseButton
                        icon-only
                        :icon="X"
                        size="xs"
                        variant="ghost"
                        aria-label="Annuler"
                        @click="editing = null"
                    />
                </template>

                <template v-else>
                    <span class="flex-1 truncate text-sm">{{ col }}</span>
                    <BaseButton
                        icon-only
                        :icon="Pencil"
                        size="xs"
                        variant="ghost"
                        :aria-label="`Renommer la colonne ${col}`"
                        title="Renommer"
                        @click="startRename(col)"
                    />
                    <BaseButton
                        icon-only
                        :icon="Trash2"
                        size="xs"
                        variant="ghost"
                        :aria-label="`Supprimer la colonne ${col}`"
                        title="Supprimer"
                        @click="confirmingDrop = col"
                    />
                </template>
            </div>
        </div>

        <FormError :message="error" />

        <div class="mt-4 flex justify-end">
            <BaseButton variant="secondary" @click="emit('close')">Fermer</BaseButton>
        </div>

        <ConfirmDialog
            v-if="confirmingDrop"
            :message="`Supprimer la colonne « ${confirmingDrop} » ? Les valeurs qu'elle contient sont perdues.`"
            confirm-label="Supprimer la colonne"
            :busy="busyColumn === confirmingDrop"
            @confirm="confirmDrop(confirmingDrop)"
            @close="confirmingDrop = null"
        />
    </Modal>
</template>
