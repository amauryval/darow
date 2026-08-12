<script setup lang="ts">
import { Bookmark, Pencil, Trash2 } from '@lucide/vue'
import type { SavedFilter } from '../../client/types'
import BaseButton from '../ui/BaseButton.vue'
import MenuDropdown from '../ui/MenuDropdown.vue'
import MenuItem from '../ui/MenuItem.vue'
import Typography from '../ui/Typography.vue'
import { useWriteGuard } from '../../composables/useWriteGuard'

defineProps<{
    filters: SavedFilter[]
    activeFilterId: string | null
    loading: boolean
    loadedOnce: boolean
    error: string
}>()
const emit = defineEmits<{
    apply: [filter: SavedFilter]
    edit: [filter: SavedFilter]
    remove: [filter: SavedFilter]
    retry: []
}>()

const { disabled: writeDisabled, reason: writeGuardReason } = useWriteGuard()
</script>

<template>
    <!-- Anchored to its button's left edge: the toolbar starts at the left of
       the panel, so a right-aligned menu ran off-screen. Applying or editing
       closes the panel; deleting leaves it open, so several filters can be
       cleaned up in one go. -->
    <MenuDropdown
        label="Filtres enregistrés"
        :icon="Bookmark"
        align="left"
        title="Filtres enregistrés"
        panel-class="max-h-80 w-80"
    >
        <template #label
            >Enregistrés<span v-if="filters.length > 0"> ({{ filters.length }})</span></template
        >

        <template #default="{ close }">
            <li v-if="loading && !loadedOnce" class="px-2 py-1.5">
                <Typography variant="body-sm">Chargement…</Typography>
            </li>

            <li
                v-else-if="error"
                class="flex-row flex-nowrap items-center justify-between gap-2 px-2 py-1.5"
            >
                <Typography variant="body-sm">{{ error }}</Typography>
                <BaseButton variant="ghost" size="xs" @click="emit('retry')">Réessayer</BaseButton>
            </li>

            <li v-else-if="filters.length === 0" class="px-2 py-1.5">
                <Typography variant="body-sm"
                    >Aucun filtre enregistré. Créez-en un avec « Filtrer ».</Typography
                >
            </li>

            <template v-else>
                <MenuItem
                    v-for="filter in filters"
                    :key="filter.filterId"
                    :active="filter.filterId === activeFilterId"
                    :title="
                        filter.filterId === activeFilterId
                            ? `Désactiver « ${filter.name} »`
                            : `Appliquer « ${filter.name} »`
                    "
                    :aria-pressed="filter.filterId === activeFilterId"
                    @click="
                        () => {
                            close()
                            emit('apply', filter)
                        }
                    "
                >
                    {{ filter.name }}
                    <template #actions>
                        <BaseButton
                            variant="ghost"
                            size="xs"
                            square
                            :icon="Pencil"
                            icon-only
                            :disabled="writeDisabled"
                            aria-label="Modifier ce filtre enregistré"
                            :title="
                                writeDisabled ? writeGuardReason : 'Modifier ce filtre enregistré'
                            "
                            @click="
                                () => {
                                    close()
                                    emit('edit', filter)
                                }
                            "
                        />
                        <BaseButton
                            variant="ghost"
                            size="xs"
                            square
                            :icon="Trash2"
                            icon-only
                            :disabled="writeDisabled"
                            class="text-base-content/40 hover:text-error"
                            aria-label="Supprimer ce filtre enregistré"
                            :title="
                                writeDisabled ? writeGuardReason : 'Supprimer ce filtre enregistré'
                            "
                            @click="emit('remove', filter)"
                        />
                    </template>
                </MenuItem>
            </template>
        </template>
    </MenuDropdown>
</template>
