<script setup lang="ts">
import { t } from '@myapplibs/web-platform/i18n'
import ToggleButton from '@myapplibs/web-platform/components/base/ToggleButton.vue'
import { icons } from '@/darow/icons'

defineProps<{ hasGeometry: boolean; showMap: boolean; showTable: boolean }>()
const emit = defineEmits<{ toggleMap: []; toggleTable: [] }>()
defineSlots<{ default?(): unknown }>()
</script>

<template>
    <div
        class="flex shrink-0 items-center gap-0.5 rounded-field border border-base-content/15 p-0.5"
        role="group"
    >
        <template v-if="hasGeometry">
            <ToggleButton
                :pressed="showTable"
                :icon="icons.Tables"
                :label="t('darow.table.grid')"
                @toggle="emit('toggleTable')"
            />
            <ToggleButton
                :pressed="showMap"
                :icon="icons.Map"
                :label="t('darow.table.map')"
                @toggle="emit('toggleMap')"
            />
            <span v-if="$slots.default" class="mx-0.5 h-5 w-px bg-base-content/15" />
        </template>
        <slot />
    </div>
</template>
