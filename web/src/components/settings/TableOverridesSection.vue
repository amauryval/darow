<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useTableSettingsOverrides } from '../../composables/useTableSettingsOverrides'
import { TABLE_SETTINGS_FIELDS, tableFieldValue } from '../../utils/settingsFields'
import type { TableSettings, TableSettingsFieldKey } from '../../client/types'
import BaseButton from '../ui/BaseButton.vue'
import Skeleton from '../ui/Skeleton.vue'
import ErrorState from '../ui/ErrorState.vue'
import Typography from '../ui/Typography.vue'

const { tables, loading, loadedOnce, error, retry, resettingTableId, resetTable } =
    useTableSettingsOverrides()

// The overridden fields of one table, labelled and valued — what the row
// actually shows, since "this table overrides something" alone answers
// nothing.
function overriddenFields(table: TableSettings) {
    return TABLE_SETTINGS_FIELDS.map((field) => ({
        label: field.label,
        value: tableFieldValue(table, field.key as TableSettingsFieldKey)
    })).filter((entry) => entry.value && entry.value.override !== null)
}
</script>

<template>
    <section class="rounded-lg border border-base-300 bg-base-100 p-4">
        <Typography variant="h4">Dérogations par table</Typography>
        <Typography variant="body-sm" class="mt-1 text-base-content/60">
            Les tables dont les paramètres s’écartent des valeurs globales ci-dessus.
        </Typography>

        <div class="mt-4">
            <Skeleton v-if="loading && !loadedOnce" :rows="2" />
            <ErrorState v-else-if="error" :message="error" @retry="retry" />
            <Typography
                v-else-if="tables.length === 0"
                variant="body-sm"
                class="text-base-content/60"
            >
                Aucune table ne modifie ces valeurs.
            </Typography>
            <ul v-else class="flex flex-col gap-2">
                <li
                    v-for="table in tables"
                    :key="table.tableId"
                    class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-base-300 px-3 py-2"
                >
                    <div class="min-w-0">
                        <RouterLink
                            :to="`/tables/${table.tableId}`"
                            class="link link-primary font-medium no-underline"
                        >
                            {{ table.tableId }}
                        </RouterLink>
                        <Typography variant="caption" class="mt-0.5 block text-base-content/60">
                            <span v-for="(entry, i) in overriddenFields(table)" :key="entry.label">
                                {{ i > 0 ? ', ' : '' }}{{ entry.label }} :
                                {{ entry.value?.override }}
                            </span>
                        </Typography>
                    </div>
                    <BaseButton
                        type="button"
                        variant="ghost"
                        size="xs"
                        :busy="resettingTableId === table.tableId"
                        :disabled="resettingTableId !== null"
                        @click="resetTable(table.tableId)"
                    >
                        Réinitialiser
                    </BaseButton>
                </li>
            </ul>
        </div>
    </section>
</template>
