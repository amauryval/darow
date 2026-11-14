<script setup lang="ts">
import { t } from '@myapplibs/web-platform/i18n'
import BaseBadge from '@myapplibs/web-platform/components/base/BaseBadge.vue'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { Filter } from '@/darow/api/filters.gen'
import { icons } from '@/darow/icons'

defineProps<{
    applied: string
    filters: Filter[]
    activeId: string
    selected: number
    selectionOnly: boolean
}>()
const emit = defineEmits<{
    open: []
    unapply: []
    toggle: [filter: Filter]
    toggleSelection: []
    clearSelection: []
}>()
</script>

<template>
    <div class="flex flex-wrap items-center gap-2">
        <BaseButton
            :tone="applied ? 'soft' : 'ghost'"
            size="sm"
            :title="t('darow.filters.title')"
            :aria-label="t('darow.filters.title')"
            @click="emit('open')"
        >
            <Icon :icon-name="icons.Filter" />
        </BaseButton>
        <BaseButton
            v-for="filter in filters"
            :key="filter.id"
            :variant="activeId === filter.id ? 'primary' : 'neutral'"
            :tone="activeId === filter.id ? 'solid' : 'outline'"
            size="sm"
            :aria-pressed="activeId === filter.id"
            :title="filter.where"
            @click="emit('toggle', filter)"
        >
            <Icon :icon-name="activeId === filter.id ? icons.Apply : icons.Filter" />
            <span class="truncate max-w-48">{{ filter.name }}</span>
        </BaseButton>
        <BaseBadge v-if="applied" variant="primary" size="md" class="gap-1" :title="applied">
            <span class="truncate max-w-64 font-mono">{{ applied }}</span>
            <BaseButton
                tone="ghost"
                size="xs"
                :title="t('darow.filters.unapply')"
                :aria-label="t('darow.filters.unapply')"
                @click="emit('unapply')"
            >
                <Icon :icon-name="icons.Remove" />
            </BaseButton>
        </BaseBadge>
        <template v-if="selected">
            <BaseButton
                :variant="selectionOnly ? 'primary' : 'neutral'"
                :tone="selectionOnly ? 'solid' : 'outline'"
                size="sm"
                :aria-pressed="selectionOnly"
                :title="t('darow.selection.only')"
                @click="emit('toggleSelection')"
            >
                <Icon :icon-name="selectionOnly ? icons.Apply : icons.Filter" />
                {{ t('darow.selection.filter', { count: selected }) }}
            </BaseButton>
            <BaseButton
                tone="ghost"
                size="sm"
                :title="t('darow.selection.clear')"
                :aria-label="t('darow.selection.clear')"
                @click="emit('clearSelection')"
            >
                <Icon :icon-name="icons.Remove" />
            </BaseButton>
        </template>
    </div>
</template>
