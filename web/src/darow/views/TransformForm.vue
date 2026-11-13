<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatNumber, t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import FormTextarea from '@myapplibs/web-platform/components/form/FormTextarea.vue'
import type { Column, TransformPreview } from '@/darow/api/engine.gen'
import { textOf } from '@/darow/utils/rowValues'

const props = defineProps<{
    columns: Column[]
    preview: TransformPreview | null
    filtered: boolean
    busy: boolean
}>()
const emit = defineEmits<{
    preview: [column: string, expression: string]
    apply: [column: string, expression: string]
}>()

const column = ref<string | number | (string | number)[]>(props.columns[0]?.name ?? '')
const expression = ref('')

const options = computed(() => props.columns.map((c) => ({ id: c.name, label: c.name })))
const ready = computed(() => String(column.value) !== '' && expression.value.trim() !== '')
const sample = computed(() =>
    (props.preview?.sample ?? []).map((row) => ({
        id: textOf(row.id),
        before: textOf(row.before),
        after: textOf(row.after)
    }))
)
</script>

<template>
    <div class="flex flex-col gap-3">
        <FormSelect v-model="column" :legend="t('darow.columns.target')" :options="options" />
        <FormTextarea
            v-model="expression"
            :legend="t('darow.columns.expression')"
            :placeholder="t('darow.columns.expressionPlaceholder')"
            :rows="3"
        />
        <p class="text-sm text-base-content/70">
            {{ filtered ? t('darow.columns.onFiltered') : t('darow.columns.onAll') }}
        </p>
        <div v-if="preview" class="flex flex-col gap-1 text-sm">
            <p>{{ t('darow.columns.matched', { count: formatNumber(preview.matched) }) }}</p>
            <table class="table table-xs">
                <thead>
                    <tr>
                        <th>{{ t('darow.table.id') }}</th>
                        <th>{{ t('darow.columns.before') }}</th>
                        <th>{{ t('darow.columns.after') }}</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="line in sample" :key="line.id">
                        <td>{{ line.id }}</td>
                        <td>{{ line.before }}</td>
                        <td>{{ line.after }}</td>
                    </tr>
                </tbody>
            </table>
        </div>
        <div class="flex justify-end gap-2">
            <BaseButton
                tone="ghost"
                size="sm"
                :disabled="!ready || busy"
                @click="emit('preview', String(column), expression)"
            >
                {{ t('darow.columns.preview') }}
            </BaseButton>
            <BaseButton
                variant="primary"
                size="sm"
                :disabled="!ready || busy"
                @click="emit('apply', String(column), expression)"
            >
                {{ t('darow.columns.apply') }}
            </BaseButton>
        </div>
    </div>
</template>
