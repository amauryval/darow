<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { ExportFormat } from '@/darow/api/darow'
import { icons } from '@/darow/icons'
import { sridOf } from '@/darow/utils/crs'

const props = defineProps<{ isOpen: boolean; hasGeometry: boolean }>()
const emit = defineEmits<{ export: [format: ExportFormat, srid: number]; close: [] }>()

const format = ref<string | number | (string | number)[]>('csv')
const srid = ref('')

watch(
    () => props.isOpen,
    (open) => {
        if (open) format.value = 'csv'
    }
)

const formats = computed(() => [
    { id: 'csv', label: t('darow.table.csv') },
    ...(props.hasGeometry ? [{ id: 'geojson', label: t('darow.table.geojson') }] : [])
])

const submit = () => emit('export', format.value as ExportFormat, sridOf(srid.value))
</script>

<template>
    <BaseModal
        :is-open="isOpen"
        :title="t('darow.export.title')"
        max-width="max-w-md"
        @close="emit('close')"
    >
        <form class="flex flex-col gap-3" @submit.prevent="submit">
            <FormSelect v-model="format" :legend="t('darow.export.format')" :options="formats" />
            <FormInput
                v-if="hasGeometry"
                v-model="srid"
                type="number"
                :label="t('darow.export.srid')"
                :description="t('darow.export.sridHint')"
            />
            <div class="flex justify-end gap-2">
                <BaseButton tone="ghost" size="sm" @click="emit('close')">{{
                    t('darow.actions.cancel')
                }}</BaseButton>
                <BaseButton type="submit" variant="primary" size="sm">
                    <Icon :icon-name="icons.Export" />
                    {{ t('darow.export.submit') }}
                </BaseButton>
            </div>
        </form>
    </BaseModal>
</template>
