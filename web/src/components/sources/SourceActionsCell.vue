<script setup lang="ts">
import { computed } from 'vue'
import { DownloadCloud, RefreshCw, Trash2 } from '@lucide/vue'
import type { SourceRow } from '../../utils/sourceRows'
import ConfirmButton from '../ui/ConfirmButton.vue'
import BaseButton from '../ui/BaseButton.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

// Icon-only actions, the same language as every other row's: the load form
// itself (CRS, table name, replace-or-create) is a modal the view owns.
const props = defineProps<{
    row: SourceRow
    // How many tables already come from this row's source, so the load action
    // says "recharger" rather than pretending it is the first time.
    loadedCount?: number
    deleting?: boolean
}>()

const emit = defineEmits<{ startLoad: []; remove: [] }>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()

// A multi-layer file is loaded through its layers, never as a whole.
const loadable = computed(() => props.row.layerCount === undefined)
// Deletion is the file's, so a layer row never offers it.
const deletable = computed(() => !props.row.layerName)

const loaded = computed(() => (props.loadedCount ?? 0) > 0)
const loadLabel = computed(() => (loaded.value ? 'Recharger ou créer une table' : 'Charger'))

const deleteMessage = computed(
    () =>
        ` « ${props.row.source.name} » du répertoire de travail ? Les tables déjà chargées depuis ce fichier sont conservées.`
)
</script>

<template>
    <div class="flex items-center justify-end gap-2">
        <BaseButton
            v-if="loadable"
            variant="ghost"
            size="xs"
            square
            icon-only
            :icon="loaded ? RefreshCw : DownloadCloud"
            :aria-label="loadLabel"
            :title="writeDisabled ? writeGuardReason : loadLabel"
            :disabled="writeDisabled"
            @click="emit('startLoad')"
        />
        <ConfirmButton
            v-if="deletable"
            label="Supprimer le fichier"
            variant="ghost"
            icon-only
            :icon="Trash2"
            :message="deleteMessage"
            :busy="deleting"
            :disabled="writeDisabled"
            :title="writeDisabled ? writeGuardReason : undefined"
            @confirm="emit('remove')"
        />
    </div>
</template>
