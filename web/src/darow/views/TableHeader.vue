<script setup lang="ts">
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import { icons } from '@/darow/icons'
import type { TableLink } from '@/darow/utils/tableLinks'
import LinksMenu from './LinksMenu.vue'

defineProps<{
    name: string
    canEdit: boolean
    canAdmin: boolean
    links: TableLink[]
}>()
const emit = defineEmits<{
    newRow: []
    columns: []
    settings: []
    rename: []
    export: []
}>()

const button = (label: string) => ({ title: label, 'aria-label': label })
</script>

<template>
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div class="group flex items-center gap-1 min-w-0">
            <h3 class="text-lg font-semibold tracking-tight truncate" :title="name">{{ name }}</h3>
            <BaseButton
                v-if="canAdmin"
                tone="ghost"
                size="xs"
                class="btn-square opacity-50 group-hover:opacity-100 focus-visible:opacity-100"
                v-bind="button(t('darow.table.rename'))"
                @click="emit('rename')"
            >
                <Icon :icon-name="icons.Rename" />
            </BaseButton>
        </div>
        <div class="flex items-center gap-1">
            <template v-if="canEdit">
                <BaseButton
                    tone="ghost"
                    size="sm"
                    class="btn-square text-success"
                    v-bind="button(t('darow.row.new'))"
                    @click="emit('newRow')"
                >
                    <Icon :icon-name="icons.Add" />
                </BaseButton>
                <BaseButton
                    tone="ghost"
                    size="sm"
                    class="btn-square"
                    v-bind="button(t('darow.columns.title'))"
                    @click="emit('columns')"
                >
                    <Icon :icon-name="icons.Columns" />
                </BaseButton>
            </template>
            <LinksMenu :links="links" />
            <BaseButton
                tone="ghost"
                size="sm"
                class="btn-square"
                v-bind="button(t('darow.export.title'))"
                @click="emit('export')"
            >
                <Icon :icon-name="icons.Export" />
            </BaseButton>
            <BaseButton
                v-if="canAdmin"
                tone="ghost"
                size="sm"
                class="btn-square"
                v-bind="button(t('darow.settings.tableTitle'))"
                @click="emit('settings')"
            >
                <Icon :icon-name="icons.Settings" />
            </BaseButton>
        </div>
    </div>
</template>
