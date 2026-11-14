<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import FormTextarea from '@myapplibs/web-platform/components/form/FormTextarea.vue'
import type { Column } from '@/darow/api/engine.gen'
import { attributeColumns, ID_COLUMN, insertToken, sqlIdentifier } from '@/darow/utils/conditions'

const props = defineProps<{ columns: Column[]; disabled?: boolean }>()
const where = defineModel<string>({ required: true })

const names = computed(() => [ID_COLUMN, ...attributeColumns(props.columns).map((c) => c.name)])

const insert = (name: string) => {
    where.value = insertToken(where.value, sqlIdentifier(name))
}
</script>

<template>
    <div class="flex flex-col gap-2">
        <FormTextarea
            v-model="where"
            :legend="t('darow.filters.where')"
            :placeholder="t('darow.filters.wherePlaceholder')"
            :disabled="disabled"
            :rows="3"
        />
        <div v-if="!disabled" class="flex flex-wrap items-center gap-1">
            <span class="text-base-content/60 mr-1 text-xs">{{
                t('darow.filters.insertColumn')
            }}</span>
            <BaseButton
                v-for="name in names"
                :key="name"
                tone="ghost"
                size="xs"
                class="font-mono"
                @click="insert(name)"
            >
                {{ name }}
            </BaseButton>
        </div>
    </div>
</template>
