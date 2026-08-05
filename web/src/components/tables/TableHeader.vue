<script setup lang="ts">
import { ref, watch } from "vue";
import { Download, GitBranch, MoreHorizontal, Pencil, Plus, Trash2, X } from "@lucide/vue";
import type { Metadata } from "../../client/types";
import Badge from "../Badge.vue";
import BaseButton from "../ui/BaseButton.vue";
import BaseInput from "../ui/BaseInput.vue";
import ConfirmDialog from "../ui/ConfirmDialog.vue";
import Divider from "../ui/Divider.vue";
import MenuDropdown from "../ui/MenuDropdown.vue";
import MenuItem from "../ui/MenuItem.vue";
import MenuTitle from "../ui/MenuTitle.vue";
import Typography from "../ui/Typography.vue";

// What the table *is* — its name, its nature, its counters — and what can be
// done to the table as an object. Nothing here acts on what is displayed:
// filtering, editing and the panel layout belong to the workspace toolbar
// below (`TableToolbar`), and mixing the two registers in one bar is what
// made this row unreadable (§8.9).
const props = defineProps<{
  table: Metadata;
  // The rename form is opened by the menu but owned by the view: it is the
  // view that knows whether the write went through, and therefore whether the
  // form may close.
  editingName: boolean;
  renaming: boolean;
  exporting: boolean;
  // How many rows are selected, on the map and in the grid alike — a counter
  // of the table, so it stands with the others rather than above the grid.
  selectedCount: number;
}>();
const emit = defineEmits<{
  "update:editingName": [editing: boolean];
  clearSelection: [];
  rename: [name: string];
  addColumn: [];
  derive: [];
  export: [];
  delete: [];
}>();

const nameInput = ref(props.table.tableName);

// Opening the form starts from the current name, whichever table it is.
watch(
  () => [props.editingName, props.table.tableName],
  () => {
    if (props.editingName) nameInput.value = props.table.tableName;
  },
  { immediate: true },
);

const confirmingDelete = ref(false);

function submitRename() {
  if (!nameInput.value.trim()) return;
  emit("rename", nameInput.value.trim());
}
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex min-w-0 flex-wrap items-center gap-2">
      <template v-if="editingName">
        <BaseInput
          v-model="nameInput"
          size="sm"
          class="w-64"
          aria-label="Nom de la table"
          @keyup.enter="submitRename"
          @keyup.esc="emit('update:editingName', false)"
        />
        <BaseButton
          size="sm"
          variant="primary"
          :busy="renaming"
          :disabled="renaming || !nameInput.trim()"
          @click="submitRename"
        >
          Enregistrer
        </BaseButton>
        <BaseButton size="sm" variant="ghost" @click="emit('update:editingName', false)">Annuler</BaseButton>
      </template>
      <template v-else>
        <Badge :tone="table.isGeo ? 'primary' : 'neutral'">{{ table.isGeo ? "Géo" : "Standard" }}</Badge>
        <Typography variant="body-sm">
          {{ table.rowCount.toLocaleString("fr-FR") }} lignes · mis à jour
          {{ new Date(table.updatedAt).toLocaleString("fr-FR") }}
        </Typography>
        <template v-if="selectedCount > 0">
          <span class="size-2.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />
          <Typography variant="body-sm">
            {{ selectedCount.toLocaleString("fr-FR") }} sélectionnée{{ selectedCount > 1 ? "s" : "" }}
          </Typography>
          <BaseButton
            size="xs"
            variant="ghost"
            :icon="X"
            icon-only
            title="Vider la sélection"
            @click="emit('clearSelection')"
          >
            Vider la sélection
          </BaseButton>
        </template>
      </template>
    </div>

    <!-- One entry point for everything that acts on the table itself: five
         buttons of five different registers side by side read as a pile, and
         four of them are used once in a table's life. -->
    <MenuDropdown label="Actions" :icon="MoreHorizontal" size="sm" align="right" panel-class="w-64">
      <template #default="{ close }">
        <MenuTitle>Structure</MenuTitle>
        <MenuItem :icon="Pencil" @click="close(); emit('update:editingName', true)">Renommer</MenuItem>
        <MenuItem :icon="Plus" @click="close(); emit('addColumn')">Ajouter une colonne</MenuItem>

        <MenuTitle>Produire</MenuTitle>
        <MenuItem :icon="GitBranch" @click="close(); emit('derive')">Dériver une table</MenuItem>
        <MenuItem :icon="Download" :disabled="exporting" @click="emit('export')">
          {{ exporting ? "Export en cours…" : "Exporter" }}
        </MenuItem>

        <li><Divider orientation="horizontal" /></li>
        <MenuItem :icon="Trash2" danger @click="close(); confirmingDelete = true">Supprimer la table</MenuItem>
      </template>
    </MenuDropdown>

    <ConfirmDialog
      v-if="confirmingDelete"
      :message="`Supprimer « ${table.tableName} » ? Les tables dérivées de celle-ci seront également supprimées.`"
      confirm-label="Supprimer la table"
      @confirm="confirmingDelete = false; emit('delete')"
      @close="confirmingDelete = false"
    />
  </div>
</template>
