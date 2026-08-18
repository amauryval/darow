<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'

// Presentational: the write, its error and which row is being renamed belong
// to whoever holds the row list, since a patched row after a rename is that
// owner's state (§0.5, §0.12) — this cell only asks for a rename.
const props = defineProps<{
    tableId: string
    tableName: string
    editing?: boolean
    renaming?: boolean
    error?: string
}>()
const emit = defineEmits<{ rename: [tableId: string, name: string]; cancel: [] }>()

const nameInput = ref(props.tableName)

watch(
    () => props.editing,
    (editing) => {
        if (editing) nameInput.value = props.tableName
    },
    { immediate: true }
)

function confirmRename() {
    const name = nameInput.value.trim()
    if (!name || name === props.tableName) {
        emit('cancel')
        return
    }
    emit('rename', props.tableId, name)
}
</script>

<template>
    <div v-if="editing" class="flex flex-col gap-1" @click.stop>
        <div class="flex items-center gap-1.5">
            <BaseInput
                v-model="nameInput"
                size="sm"
                @keyup.enter="confirmRename"
                @keyup.esc="emit('cancel')"
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
            <BaseButton size="sm" variant="secondary" :disabled="renaming" @click="emit('cancel')"
                >Annuler</BaseButton
            >
        </div>
        <p v-if="error" class="text-xs text-error">{{ error }}</p>
    </div>
    <RouterLink
        v-else
        :to="`/tables/${tableId}`"
        class="link link-primary font-medium no-underline"
        @click.stop
        >{{ tableName }}</RouterLink
    >
</template>
