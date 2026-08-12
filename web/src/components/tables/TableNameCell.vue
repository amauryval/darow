<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Pencil } from '@lucide/vue'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// Presentational: the write and its error belong to whoever holds the row
// list, since a patched row after a rename is that owner's state (§0.5,
// §0.12) — this cell only asks for a rename and reflects whether one is in
// flight or failed for its own row.
const props = defineProps<{
    tableId: string
    tableName: string
    renaming?: boolean
    error?: string
}>()
const emit = defineEmits<{ rename: [tableId: string, name: string] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

const editing = ref(false)
const nameInput = ref('')

function startRename() {
    nameInput.value = props.tableName
    editing.value = true
}

function confirmRename() {
    const name = nameInput.value.trim()
    if (!name || name === props.tableName) {
        editing.value = false
        return
    }
    emit('rename', props.tableId, name)
}

// Closes the form once the caller's write has landed — a failed one leaves
// it open, with the error under the field (§8.4), rather than editing
// vanishing under a rejected name.
watch(
    () => props.renaming,
    (renaming, wasRenaming) => {
        if (wasRenaming && !renaming && !props.error) editing.value = false
    }
)
</script>

<template>
    <div v-if="editing" class="flex flex-col gap-1" @click.stop>
        <div class="flex items-center gap-1.5">
            <BaseInput
                v-model="nameInput"
                size="sm"
                @keyup.enter="confirmRename"
                @keyup.esc="editing = false"
            />
            <BaseButton
                size="sm"
                variant="primary"
                :busy="renaming"
                :disabled="renaming || !nameInput.trim()"
                @click="confirmRename"
            >
                Enregistrer
            </BaseButton>
            <BaseButton size="sm" variant="secondary" :disabled="renaming" @click="editing = false"
                >Annuler</BaseButton
            >
        </div>
        <p v-if="error" class="text-xs text-error">{{ error }}</p>
    </div>
    <div v-else class="group flex items-center gap-1.5">
        <RouterLink
            :to="`/tables/${tableId}`"
            class="link link-primary font-medium no-underline"
            @click.stop
            >{{ tableName }}</RouterLink
        >
        <BaseButton
            variant="ghost"
            size="xs"
            square
            :icon="Pencil"
            icon-only
            aria-label="Renommer la table"
            :disabled="writeDisabled"
            :title="writeDisabled ? writeGuardReason : undefined"
            class="opacity-0 group-hover:opacity-100"
            @click.stop="startRename"
        />
    </div>
</template>
