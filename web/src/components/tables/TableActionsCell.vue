<script setup lang="ts">
import { Pencil, Trash2 } from '@lucide/vue'
import ConfirmButton from '../ui/ConfirmButton.vue'
import BaseButton from '../ui/BaseButton.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// A row's actions, icon-only like every other list's (§8.9): the rename opens
// the inline editor in the name cell, the delete confirms first (§8.3).
const props = defineProps<{ tableName: string; removing?: boolean }>()

const emit = defineEmits<{ rename: []; remove: [] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()
</script>

<template>
    <div class="flex items-center justify-end gap-2">
        <BaseButton
            variant="ghost"
            size="xs"
            square
            icon-only
            :icon="Pencil"
            aria-label="Renommer la table"
            :title="writeDisabled ? writeGuardReason : 'Renommer la table'"
            :disabled="writeDisabled"
            @click.stop="emit('rename')"
        />
        <ConfirmButton
            label="Supprimer la table"
            variant="ghost"
            icon-only
            :icon="Trash2"
            :message="` « ${props.tableName} » ? Ses données, ses paramètres et ses filtres enregistrés sont supprimés avec elle. Le fichier source, lui, reste dans le répertoire de travail.`"
            :busy="removing"
            :disabled="writeDisabled"
            :title="writeDisabled ? writeGuardReason : undefined"
            @confirm="emit('remove')"
        />
    </div>
</template>
