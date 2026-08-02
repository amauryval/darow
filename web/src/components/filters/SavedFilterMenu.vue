<script setup lang="ts">
import { ref, useId } from "vue";
import { onClickOutside } from "@vueuse/core";
import { Bookmark, Pencil, Trash2 } from "@lucide/vue";
import type { SavedFilter } from "../../client/types";
import BaseButton from "../ui/BaseButton.vue";
import Typography from "../ui/Typography.vue";

defineProps<{
  filters: SavedFilter[];
  activeFilterId: string | null;
  loading: boolean;
  loadedOnce: boolean;
  error: string;
}>();
const emit = defineEmits<{
  apply: [filter: SavedFilter];
  edit: [filter: SavedFilter];
  remove: [filter: SavedFilter];
  retry: [];
}>();

// Open state is held here rather than left to daisyUI's :focus-within
// dropdown: that variant only closes when focus leaves, so applying a filter
// left the panel hanging over the toolbar, and opening the editor put a modal
// behind a still-open menu.
const root = ref<HTMLElement | null>(null);
const open = ref(false);
const panelId = useId();

onClickOutside(root, () => (open.value = false));

// Applying or editing closes the panel; deleting leaves it open, so several
// filters can be cleaned up in one go.
function onApply(filter: SavedFilter) {
  open.value = false;
  emit("apply", filter);
}

function onEdit(filter: SavedFilter) {
  open.value = false;
  emit("edit", filter);
}
</script>

<template>
  <div ref="root" class="relative" @keydown.esc="open = false">
    <BaseButton
      variant="secondary"
      size="sm"
      :icon="Bookmark"
      :aria-expanded="open"
      :aria-controls="panelId"
      aria-haspopup="true"
      title="Filtres enregistrés"
      @click="open = !open"
    >
      Enregistrés<span v-if="filters.length > 0"> ({{ filters.length }})</span>
    </BaseButton>

    <!-- Anchored to the button's left edge: the toolbar starts at the left of
         the panel, so a right-aligned menu would extend off-screen. -->
    <div
      v-if="open"
      :id="panelId"
      class="absolute left-0 top-full z-30 mt-1 max-h-80 w-80 overflow-y-auto rounded-lg border border-base-300 bg-base-100 p-1 shadow-lg"
    >
      <Typography v-if="loading && !loadedOnce" variant="body-sm" class="block px-2 py-1.5">Chargement…</Typography>

      <div v-else-if="error" class="flex items-center justify-between gap-2 px-2 py-1.5">
        <Typography variant="body-sm">{{ error }}</Typography>
        <BaseButton variant="ghost" size="xs" @click="emit('retry')">Réessayer</BaseButton>
      </div>

      <Typography v-else-if="filters.length === 0" variant="body-sm" class="block px-2 py-1.5">
        Aucun filtre enregistré. Créez-en un avec « Filtrer ».
      </Typography>

      <ul v-else class="flex flex-col">
        <li v-for="filter in filters" :key="filter.filterId" class="flex items-center gap-0.5">
          <button
            type="button"
            class="min-w-0 flex-1 truncate rounded px-2 py-1.5 text-left text-sm hover:bg-base-200"
            :class="{ 'font-semibold text-primary': filter.filterId === activeFilterId }"
            :aria-pressed="filter.filterId === activeFilterId"
            :title="filter.filterId === activeFilterId ? `Désactiver « ${filter.name} »` : `Appliquer « ${filter.name} »`"
            @click="onApply(filter)"
          >
            {{ filter.name }}
          </button>
          <BaseButton
            variant="ghost"
            size="xs"
            square
            :icon="Pencil"
            icon-only
            aria-label="Modifier ce filtre enregistré"
            title="Modifier ce filtre enregistré"
            @click="onEdit(filter)"
          />
          <BaseButton
            variant="ghost"
            size="xs"
            square
            :icon="Trash2"
            icon-only
            class="text-base-content/40 hover:text-error"
            aria-label="Supprimer ce filtre enregistré"
            title="Supprimer ce filtre enregistré"
            @click="emit('remove', filter)"
          />
        </li>
      </ul>
    </div>
  </div>
</template>
