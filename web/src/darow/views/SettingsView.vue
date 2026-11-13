<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import Card from '@myapplibs/web-platform/components/base/Card.vue'
import PageTemplate from '@myapplibs/web-platform/components/layout/PageTemplate.vue'
import { useSettings } from '@/darow/composables/useSettings'
import { icons } from '@/darow/icons'
import {
    formOfEffective,
    formOfValues,
    valuesOfForm,
    type SettingsForm
} from '@/darow/utils/settingsForm'
import SettingsFields from './SettingsFields.vue'

const { global, busy, loadGlobal, saveGlobal } = useSettings()
const form = ref<SettingsForm>(formOfValues({}))

onMounted(loadGlobal)

watch(global, (effective) => {
    if (effective) form.value = formOfEffective(effective)
})

const breadcrumbs = computed(() => [{ label: t('darow.pages.settings'), icon: icons.Settings }])
</script>

<template>
    <PageTemplate :breadcrumbs="breadcrumbs">
        <Card>
            <form class="flex flex-col gap-3" @submit.prevent="saveGlobal(valuesOfForm(form))">
                <p class="text-sm text-base-content/70">{{ t('darow.settings.globalHint') }}</p>
                <SettingsFields v-model="form" :disabled="busy" />
                <div class="flex justify-end">
                    <BaseButton type="submit" variant="primary" size="sm" :disabled="busy">
                        {{ t('darow.actions.save') }}
                    </BaseButton>
                </div>
            </form>
        </Card>
    </PageTemplate>
</template>
