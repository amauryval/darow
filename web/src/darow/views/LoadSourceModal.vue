<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import FormSelect from '@myapplibs/web-platform/components/form/FormSelect.vue'
import type { LoadRequest } from '@/darow/api/rest.gen'
import { sridOf } from '@/darow/utils/crs'
import { tableNameOf } from '@/darow/utils/tableName'

const props = defineProps<{ source: string; layers: string[] }>()
const emit = defineEmits<{ load: [request: LoadRequest]; close: [] }>()

const table = ref('')
const layer = ref<string | number | (string | number)[]>('')
const srid = ref('')

watch(
    () => props.source,
    (source) => {
        table.value = tableNameOf(source)
        layer.value = props.layers[0] ?? ''
        srid.value = ''
    }
)

const layerOptions = computed(() => props.layers.map((name) => ({ id: name, label: name })))

const submit = () =>
    emit('load', {
        table: table.value.trim(),
        layer: String(layer.value),
        srid: sridOf(srid.value)
    })
</script>

<template>
    <BaseModal
        :is-open="source !== ''"
        :title="t('darow.sources.loadTitle', { name: source })"
        max-width="max-w-md"
        @close="emit('close')"
    >
        <form class="flex flex-col gap-3" @submit.prevent="submit">
            <FormInput
                v-model="table"
                :label="t('darow.sources.table')"
                :description="t('darow.sources.tableHint')"
            />
            <FormSelect
                v-if="layers.length > 1"
                v-model="layer"
                :legend="t('darow.sources.layer')"
                :options="layerOptions"
            />
            <FormInput
                v-model="srid"
                type="number"
                :label="t('darow.sources.srid')"
                :description="t('darow.sources.sridHint')"
            />
            <div class="flex justify-end gap-2">
                <BaseButton tone="ghost" size="sm" @click="emit('close')">{{
                    t('darow.actions.cancel')
                }}</BaseButton>
                <BaseButton type="submit" variant="primary" size="sm" :disabled="!table.trim()">
                    {{ t('darow.sources.load') }}
                </BaseButton>
            </div>
        </form>
    </BaseModal>
</template>
