<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Geometry } from 'geojson'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseModal from '@myapplibs/web-platform/components/base/BaseModal.vue'
import FormInput from '@myapplibs/web-platform/components/form/FormInput.vue'
import Icon from '@myapplibs/web-platform/components/style/Icon.vue'
import type { Column, Extent } from '@/darow/api/engine.gen'
import type { RowRequest } from '@/darow/api/rest.gen'
import { icons } from '@/darow/icons'
import { geometryOf, sameGeometry } from '@/darow/utils/geometry'
import { changedValues, draftOf, rowIdOf } from '@/darow/utils/rowValues'
import GeometryEditor from './GeometryEditor.vue'
import RowWkt from './RowWkt.vue'

const props = defineProps<{
    row: Record<string, unknown> | null
    columns: Column[]
    hasGeometry: boolean
    canEdit: boolean
    tilesUrl: string
    extent?: Extent
}>()
const emit = defineEmits<{
    save: [id: number | null, request: RowRequest]
    remove: [id: number]
    close: []
}>()

const draft = ref<Record<string, string>>({})
const geometry = ref<Geometry | null>(null)

watch(
    () => props.row,
    (row) => {
        draft.value = row ? draftOf(props.columns, row) : {}
        geometry.value = row ? geometryOf(row) : null
    },
    { immediate: true }
)

const id = computed(() => (props.row ? rowIdOf(props.row) : null))
const values = computed(() =>
    props.row ? changedValues(props.columns, props.row, draft.value) : {}
)
const geometryChanged = computed(
    () => props.hasGeometry && !!props.row && !sameGeometry(geometry.value, geometryOf(props.row))
)
const dirty = computed(
    () => id.value === null || geometryChanged.value || Object.keys(values.value).length > 0
)
const title = computed(() =>
    id.value === null ? t('darow.row.new') : t('darow.row.title', { id: String(id.value) })
)

const submit = () =>
    emit('save', id.value, {
        values: values.value,
        ...(geometryChanged.value ? { geometry: geometry.value } : {})
    })
</script>

<template>
    <BaseModal :is-open="row !== null" :title="title" max-width="max-w-5xl" @close="emit('close')">
        <form class="flex flex-col gap-3" @submit.prevent="submit">
            <div class="grid gap-3 md:grid-cols-2">
                <FormInput
                    v-for="column in columns"
                    :key="column.name"
                    v-model="draft[column.name]"
                    :label="column.name"
                    :description="column.type"
                    :disabled="!canEdit"
                />
            </div>
            <GeometryEditor
                v-if="hasGeometry && row"
                :key="id ?? 'new'"
                v-model="geometry"
                :disabled="!canEdit"
                :tiles-url="tilesUrl"
                :extent="extent"
                :row-id="id"
            />
            <RowWkt v-if="hasGeometry && id !== null" :key="id" :id="id" />
            <div v-if="canEdit" class="flex justify-between gap-2">
                <BaseButton
                    v-if="id !== null"
                    tone="ghost"
                    size="sm"
                    :title="t('darow.row.delete')"
                    :aria-label="t('darow.row.delete')"
                    @click="emit('remove', id)"
                >
                    <Icon :icon-name="icons.Delete" />
                </BaseButton>
                <BaseButton
                    type="submit"
                    variant="primary"
                    size="sm"
                    class="ml-auto"
                    :disabled="!dirty"
                >
                    <Icon :icon-name="icons.Save" />
                    {{ t('darow.actions.save') }}
                </BaseButton>
            </div>
        </form>
    </BaseModal>
</template>
