<script setup lang="ts">
import { ref } from 'vue'
import { useAddColumn } from '../../composables/useAddColumn'
import Modal from '../ui/Modal.vue'
import BaseInput from '../ui/BaseInput.vue'
import FormError from '../ui/FormError.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'

// The write is the caller's useTableMetadata instance (onAdd): it already
// owns the table's schema and reloads it after adding a column, so a second
// call here would be a second owner of the same state (§0.5, §0.12).
const props = defineProps<{ onAdd: (name: string, type: string) => Promise<void> }>()
const emit = defineEmits<{ close: [] }>()

const { isLoading, error, confirm } = useAddColumn(props.onAdd)
const name = ref('')
const type = ref('VARCHAR')

function openModal() {
    name.value = ''
    type.value = 'VARCHAR'
    error.value = ''
}

async function confirmAdd() {
    if (await confirm(name.value, type.value)) emit('close')
}

function onClose() {
    emit('close')
}

defineExpose({ openModal })
</script>

<template>
    <Modal title="Ajouter une colonne" :dirty="!!name.trim()" @close="onClose" @submit="confirmAdd">
        <BaseInput v-model="name" label="Nom" class="mb-3" />
        <BaseInput
            v-model="type"
            label="Type"
            placeholder="VARCHAR, INTEGER, DOUBLE…"
            class="font-mono"
        />
        <FormError :message="error" />
        <ModalFormActions
            :confirm-label="isLoading ? 'Ajout…' : 'Ajouter'"
            :busy="isLoading"
            :disabled="isLoading || !name.trim()"
            @cancel="onClose"
            @confirm="confirmAdd"
        />
    </Modal>
</template>
