<script setup lang="ts">
import { t } from '@myapplibs/web-platform/i18n'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import { SETTING_FIELDS, type SettingsForm } from '@/darow/utils/settingsForm'

defineProps<{ placeholders?: SettingsForm; disabled: boolean }>()
const form = defineModel<SettingsForm>({ required: true })
</script>

<template>
    <div class="grid gap-3 sm:grid-cols-2">
        <FormInput
            v-for="field in SETTING_FIELDS"
            :key="field"
            v-model="form[field]"
            type="number"
            :label="t(`darow.settings.fields.${field}`)"
            :description="t(`darow.settings.hints.${field}`)"
            :placeholder="placeholders?.[field] ?? ''"
            :disabled="disabled"
        />
    </div>
</template>
