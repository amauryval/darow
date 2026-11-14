<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSection from '@myapplibs/web-platform/components/form/FormSection.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { Column } from '@/darow/api/engine.gen'
import { icons } from '@/darow/icons'
import WhereEditor from './WhereEditor.vue'

const props = defineProps<{
    isOpen: boolean
    columns: Column[]
    pending: boolean
    canEdit: boolean
}>()
const where = defineModel<string>('where', { required: true })
const emit = defineEmits<{
    apply: []
    save: [name: string]
    manage: []
    close: []
}>()

const name = ref('')
watch(
    () => props.isOpen,
    (open) => {
        if (open) name.value = ''
    }
)

const canSave = computed(() => name.value.trim() !== '' && where.value.trim() !== '')
</script>

<template>
    <BaseModal
        :is-open="isOpen"
        :title="t('darow.filters.title')"
        max-width="max-w-3xl"
        @close="emit('close')"
    >
        <div class="flex flex-col gap-4">
            <FormSection :title="t('darow.filters.conditions')">
                <WhereEditor v-model="where" :columns="columns" />
                <div class="flex justify-between">
                    <BaseButton tone="ghost" size="sm" @click="emit('manage')">
                        <Icon :icon-name="icons.Settings" />
                        {{ t('darow.filters.manage') }}
                    </BaseButton>
                    <BaseButton
                        variant="primary"
                        size="sm"
                        :disabled="!pending"
                        @click="emit('apply')"
                    >
                        <Icon :icon-name="icons.Apply" />
                        {{ t('darow.filters.apply') }}
                    </BaseButton>
                </div>
            </FormSection>
            <FormSection v-if="canEdit" :title="t('darow.filters.saveDraft')">
                <form class="flex items-end gap-2" @submit.prevent="emit('save', name.trim())">
                    <FormInput
                        v-model="name"
                        class="flex-1"
                        size="sm"
                        :placeholder="t('darow.filters.name')"
                    />
                    <BaseButton
                        type="submit"
                        tone="soft"
                        size="sm"
                        :disabled="!canSave"
                        :title="t('darow.filters.save')"
                        :aria-label="t('darow.filters.save')"
                    >
                        <Icon :icon-name="icons.Save" />
                    </BaseButton>
                </form>
            </FormSection>
        </div>
    </BaseModal>
</template>
