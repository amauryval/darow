<script setup lang="ts">
import { ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import type { TableSettings, Values } from '@/darow/api/settings.gen'
import {
    formOfEffective,
    formOfValues,
    valuesOfForm,
    type SettingsForm
} from '@/darow/utils/settingsForm'
import SettingsFields from './SettingsFields.vue'

const props = defineProps<{ settings: TableSettings | null; busy: boolean }>()
const emit = defineEmits<{ save: [values: Values]; close: [] }>()

const form = ref<SettingsForm>(formOfValues({}))

watch(
    () => props.settings,
    (settings) => {
        form.value = formOfValues(settings?.overrides ?? {})
    },
    { immediate: true }
)
</script>

<template>
    <BaseModal
        :is-open="settings !== null"
        :title="t('darow.settings.tableTitle')"
        max-width="max-w-2xl"
        @close="emit('close')"
    >
        <form
            v-if="settings"
            class="flex flex-col gap-3"
            @submit.prevent="emit('save', valuesOfForm(form))"
        >
            <p class="text-sm text-base-content/70">{{ t('darow.settings.tableHint') }}</p>
            <SettingsFields
                v-model="form"
                :placeholders="formOfEffective(settings.effective)"
                :disabled="busy"
            />
            <div class="flex justify-end">
                <BaseButton type="submit" variant="primary" size="sm" :disabled="busy">
                    {{ t('darow.actions.save') }}
                </BaseButton>
            </div>
        </form>
    </BaseModal>
</template>
