<script setup lang="ts">
import { computed, ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSection from '@myapplibs/web-platform/components/form/FormSection.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { Column, TransformPreview } from '@/darow/api/engine.gen'
import { COLUMN_TYPES } from '@/darow/utils/catalog'
import { icons } from '@/darow/icons'
import NameModal from './NameModal.vue'
import TransformForm from './TransformForm.vue'

defineProps<{
    isOpen: boolean
    columns: Column[]
    preview: TransformPreview | null
    filtered: boolean
    busy: boolean
}>()
const emit = defineEmits<{
    add: [name: string, type: string]
    rename: [column: string, to: string]
    drop: [column: string]
    preview: [column: string, expression: string]
    transform: [column: string, expression: string]
    close: []
}>()

const name = ref('')
const type = ref<string | number | (string | number)[]>(COLUMN_TYPES[0])
const renaming = ref('')

const typeOptions = computed(() => COLUMN_TYPES.map((id) => ({ id, label: id })))

const add = () => {
    emit('add', name.value.trim(), String(type.value))
    name.value = ''
}

const onRename = (to: string) => {
    emit('rename', renaming.value, to)
    renaming.value = ''
}
</script>

<template>
    <BaseModal
        :is-open="isOpen"
        :title="t('darow.columns.title')"
        max-width="max-w-3xl"
        @close="emit('close')"
    >
        <div class="flex flex-col gap-4">
            <FormSection :title="t('darow.columns.list')">
                <div
                    v-for="column in columns"
                    :key="column.name"
                    class="flex items-center justify-between gap-2"
                >
                    <span class="truncate"
                        >{{ column.name }}
                        <span class="text-base-content/60 text-xs">{{ column.type }}</span></span
                    >
                    <div class="flex gap-1">
                        <BaseButton
                            tone="ghost"
                            size="xs"
                            :title="t('darow.columns.rename')"
                            :aria-label="t('darow.columns.rename')"
                            @click="renaming = column.name"
                        >
                            <Icon :icon-name="icons.Rename" />
                        </BaseButton>
                        <BaseButton
                            tone="ghost"
                            size="xs"
                            :title="t('darow.columns.delete')"
                            :aria-label="t('darow.columns.delete')"
                            @click="emit('drop', column.name)"
                        >
                            <Icon :icon-name="icons.Delete" />
                        </BaseButton>
                    </div>
                </div>
            </FormSection>
            <FormSection :title="t('darow.columns.add')">
                <form class="flex flex-wrap items-end gap-2" @submit.prevent="add">
                    <FormInput v-model="name" :label="t('darow.columns.name')" />
                    <FormSelect
                        v-model="type"
                        :legend="t('darow.columns.type')"
                        :options="typeOptions"
                    />
                    <BaseButton
                        type="submit"
                        variant="success"
                        size="sm"
                        :disabled="!name.trim() || busy"
                        :title="t('darow.columns.add')"
                        :aria-label="t('darow.columns.add')"
                    >
                        <Icon :icon-name="icons.Add" />
                    </BaseButton>
                </form>
            </FormSection>
            <FormSection v-if="columns.length" :title="t('darow.columns.transform')">
                <TransformForm
                    :columns="columns"
                    :preview="preview"
                    :filtered="filtered"
                    :busy="busy"
                    @preview="(column, expression) => emit('preview', column, expression)"
                    @apply="(column, expression) => emit('transform', column, expression)"
                />
            </FormSection>
        </div>
        <NameModal
            :is-open="renaming !== ''"
            :title="t('darow.columns.rename')"
            :label="t('darow.columns.name')"
            :initial="renaming"
            @submit="onRename"
            @close="renaming = ''"
        />
    </BaseModal>
</template>
