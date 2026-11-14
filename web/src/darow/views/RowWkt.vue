<script setup lang="ts">
import { onMounted } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSection from '@myapplibs/web-platform/components/form/FormSection.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import { useRowWkt } from '@/darow/composables/useRowWkt'
import { useTable } from '@/darow/composables/useTable'
import { icons } from '@/darow/icons'

const props = defineProps<{ id: number }>()

const { name } = useTable()
const { srid, wkt, busy, load, copy } = useRowWkt(name)

onMounted(() => load(props.id))
</script>

<template>
    <FormSection :title="t('darow.wkt.title')">
        <div class="flex items-end gap-2">
            <FormInput
                v-model="srid"
                type="number"
                :label="t('darow.wkt.srid')"
                :description="t('darow.wkt.sridHint')"
            />
            <BaseButton size="sm" :loading="busy" @click="load(id)">
                {{ t('darow.wkt.project') }}
            </BaseButton>
            <BaseButton
                tone="ghost"
                size="sm"
                :disabled="!wkt"
                :title="t('darow.wkt.copy')"
                :aria-label="t('darow.wkt.copy')"
                @click="copy"
            >
                <Icon :icon-name="icons.Copy" />
            </BaseButton>
        </div>
        <pre
            class="text-xs whitespace-pre-wrap break-all rounded-lg bg-base-100 p-2 max-h-48 overflow-auto"
            >{{ wkt ?? t('darow.wkt.empty') }}</pre
        >
    </FormSection>
</template>
