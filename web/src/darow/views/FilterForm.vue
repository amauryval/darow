<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import type { Column, Table } from '@/darow/api/engine.gen'
import type { FilterDraft } from '@/darow/composables/useFilterEditor'
import WhereEditor from './WhereEditor.vue'

const props = defineProps<{ tables: Table[]; columns: Column[]; canEdit: boolean }>()
const draft = defineModel<FilterDraft>({ required: true })
const emit = defineEmits<{ table: [name: string] }>()

const tableOptions = computed(() =>
    props.tables.map((table) => ({ id: table.name, label: table.name }))
)

const name = computed({
    get: () => draft.value.name,
    set: (value: string) => {
        draft.value = { ...draft.value, name: value }
    }
})

const where = computed({
    get: () => draft.value.where,
    set: (value: string) => {
        draft.value = { ...draft.value, where: value }
    }
})
</script>

<template>
    <div class="flex flex-col gap-4">
        <FormInput v-model="name" :label="t('darow.filters.name')" :disabled="!canEdit" />
        <FormSelect
            :model-value="draft.table"
            :legend="t('darow.filters.table')"
            :options="tableOptions"
            :placeholder="t('darow.filters.allTables')"
            :disabled="!canEdit"
            @update:model-value="emit('table', String($event))"
        />
        <WhereEditor v-model="where" :columns="columns" :disabled="!canEdit" />
    </div>
</template>
