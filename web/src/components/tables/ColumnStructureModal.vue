<script setup lang="ts">
import { ref } from 'vue'
import { Check, Pencil, Trash2, X } from '@lucide/vue'
import { errorMessage } from '../../client/http'
import { useToast } from '../../composables/useToast'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import ConfirmDialog from '../ui/ConfirmDialog.vue'
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

const toast = useToast()
const editing = ref<string | null>(null)
const draftName = ref('')
const confirmingDrop = ref<string | null>(null)
const busyColumn = ref<string | null>(null)
const error = ref<string | null>(null)

function startRename(col: string) {
    editing.value = col
    draftName.value = col
    error.value = null
}

async function confirmRename(col: string) {
    const name = draftName.value.trim()
    if (!name || name === col) {
        editing.value = null
        return
    }
    busyColumn.value = col
    error.value = null
    try {
        await props.onRename(col, name)
        editing.value = null
        toast.success('Colonne renommée.')
    } catch (e) {
        error.value = errorMessage(e, 'Impossible de renommer la colonne')
    } finally {
        busyColumn.value = null
    }
}

async function confirmDrop(col: string) {
    busyColumn.value = col
    error.value = null
    try {
        await props.onDrop(col)
        confirmingDrop.value = null
        toast.success('Colonne supprimée.')
    } catch (e) {
        confirmingDrop.value = null
        error.value = errorMessage(e, 'Impossible de supprimer la colonne')
    } finally {
        busyColumn.value = null
    }
}
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

        <p v-if="error" class="mt-3 text-xs text-error">{{ error }}</p>

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
