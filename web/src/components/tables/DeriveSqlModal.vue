<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDeriveFromQuery } from '../../composables/useDeriveFromQuery'
import { useToast } from '../../composables/useToast'
import { useWriteGuard } from '../../composables/useWriteGuard'
import Modal from '../ui/Modal.vue'
import DeriveSqlForm from './DeriveSqlForm.vue'
import BaseInput from '../ui/BaseInput.vue'
import FormError from '../ui/FormError.vue'
import ModalFormActions from '../ui/ModalFormActions.vue'
import Typography from '../ui/Typography.vue'

// The free-SQL mode alone, opened from the catalogue (`TablesView`) rather
// than from one table's page: the query runs against every registered table,
// so it belongs next to the catalogue it actually acts on (TODO-F38).
const emit = defineEmits<{ close: []; derived: [{ tableId: string }] }>()

const toast = useToast()
const { isLoading, error, submit } = useDeriveFromQuery()
const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

const name = ref('')
const query = ref('')

const canSubmit = computed(
    () => name.value.trim() !== '' && query.value.trim() !== '' && !isLoading.value
)

// Anything the user has actually put in is worth a confirmation before a
// stray backdrop click throws it away (§8.3).
const dirty = computed(() => name.value.trim() !== '' || query.value.trim() !== '')

async function onSubmit() {
    if (!canSubmit.value) return
    const result = await submit(query.value.trim(), name.value.trim())
    // A null result means the call failed and `error` already carries the
    // message — it stays in front of the user rather than going out as a
    // toast (§8.4).
    if (result === null) return

    emit('derived', result)
    emit('close')
    toast.success('Table dérivée créée.')
}
</script>

<template>
    <Modal
        title="Dériver une table par SQL"
        xl
        :dirty="dirty"
        @close="emit('close')"
        @submit="onSubmit"
    >
        <DeriveSqlForm v-model="query" />

        <div class="mt-4 border-t border-base-300 pt-4">
            <BaseInput
                v-model="name"
                size="sm"
                label="Nom de la nouvelle table"
                placeholder="Nom de la nouvelle table"
            />
            <Typography variant="caption" class="mt-1 block">
                La table dérivée est un instantané figé : elle n'est jamais resynchronisée avec sa
                source.
            </Typography>
        </div>

        <FormError :message="error" />

        <ModalFormActions
            :confirm-label="isLoading ? 'Création…' : 'Créer'"
            :busy="isLoading"
            :disabled="!canSubmit || writeDisabled"
            :reason="writeDisabled ? writeGuardReason : undefined"
            @cancel="emit('close')"
            @confirm="onSubmit"
        />
    </Modal>
</template>
