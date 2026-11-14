<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import type { Map } from 'maplibre-gl'
import { t } from '@myapplibs/web-platform/i18n'
import BaseButton from '@myapplibs/web-platform/components/base/BaseButton.vue'
import BaseMap from '@myapplibs/web-platform/components/map/BaseMap.vue'
import GeometryEditToolbar from '@myapplibs/web-platform/components/map/GeometryEditToolbar.vue'
import PickToolbar from '@myapplibs/web-platform/components/map/PickToolbar.vue'
import type { PickMode } from '@myapplibs/web-platform/components/map/pickIds'
import { idleCursor } from '@myapplibs/web-platform/components/map/pointPicking'
import { useGeometryEditor } from '@myapplibs/web-platform/components/map/useGeometryEditor'
import { fitExtent } from '@myapplibs/web-platform/components/map/fitPositions'
import { overEditHandle } from '@myapplibs/web-platform/components/map/geometryEditing'
import { bindVectorPicking } from '@myapplibs/web-platform/components/map/vectorPicking'
import {
    addVectorLayers,
    hideVectorFeatures,
    setVectorSelection,
    setVectorTiles
} from '@myapplibs/web-platform/components/map/vectorTiles'
import type { Extent } from '@/darow/api/engine.gen'
import { useRowGeometries } from '@/darow/composables/useRowGeometries'
import { useMapEditing } from '@/darow/composables/useMapEditing'
import { useTable } from '@/darow/composables/useTable'
import { icons } from '@/darow/icons'
import { TABLE_LAYERS } from '@/darow/utils/tableLayers'

const props = defineProps<{
    tilesUrl: string
    extent?: Extent
    selected: number[]
    canEdit: boolean
}>()
const emit = defineEmits<{
    select: [id: number]
    pick: [ids: number[], mode: PickMode]
    clear: []
}>()

const map = shallowRef<Map | null>(null)
const selecting = ref(false)
const mapEdit = useMapEditing()
const { name } = useTable()
const exact = useRowGeometries(() => name.value)
const hidden = mapEdit.hidden
const editor = useGeometryEditor(mapEdit.geometry, {
    snapFeatures: (box) => (map.value ? exact.near(map.value, box, hidden.value) : []),
    neighbours: mapEdit.neighbours
})
let unbind: (() => void) | null = null

const recenter = () => {
    if (map.value) fitExtent(map.value, props.extent)
}

const onReady = (ready: Map) => {
    map.value = ready
    addVectorLayers(ready, TABLE_LAYERS, props.tilesUrl, props.selected)
    recenter()
    unbind = bindVectorPicking(ready, {
        layers: TABLE_LAYERS.layers,
        selecting: () => selecting.value && !mapEdit.editing.value,
        onSelect: (id, event) => {
            const used = !!editor.drawing.value || overEditHandle(ready, event.point)
            if (!mapEdit.click(id, used)) emit('select', id)
        },
        onPick: (ids, mode) => emit('pick', ids, mode)
    })
}

const onStyle = (ready: Map) => {
    addVectorLayers(ready, TABLE_LAYERS, props.tilesUrl, props.selected)
    hideVectorFeatures(ready, TABLE_LAYERS, hidden.value)
    if (mapEdit.editing.value) editor.restyle(ready)
}

watch(
    () => props.tilesUrl,
    (url) => {
        exact.clear()
        if (map.value) setVectorTiles(map.value, TABLE_LAYERS, url)
    }
)

watch(
    () => props.selected,
    (ids) => {
        if (map.value) setVectorSelection(map.value, TABLE_LAYERS, ids)
    }
)

watch(selecting, (on) => {
    if (map.value) map.value.getCanvas().style.cursor = idleCursor(on)
})

watch(mapEdit.editing, (on) => {
    if (!map.value) return
    if (on) editor.attach(map.value)
    else editor.detach()
})

watch(hidden, (ids) => {
    if (map.value) hideVectorFeatures(map.value, TABLE_LAYERS, ids)
})

watch(mapEdit.rowId, () => {
    if (map.value && mapEdit.geometry.value)
        exact.around(map.value, mapEdit.geometry.value, hidden.value)
})

const onSave = async () => {
    if (await mapEdit.save()) editor.forget()
}

onBeforeUnmount(() => unbind?.())
</script>

<template>
    <div class="relative rounded-box overflow-hidden">
        <BaseMap :zoom="1" @map-ready="onReady" @style-ready="onStyle" />
        <div class="absolute left-2 top-2 z-20 flex flex-col items-start gap-2">
            <PickToolbar
                :selecting="selecting"
                :count="selected.length"
                :title="t('darow.selection.mapHint')"
                @toggle="selecting = !selecting"
                @clear="emit('clear')"
            >
                <BaseButton
                    v-if="extent"
                    size="xs"
                    tone="soft"
                    variant="primary"
                    :title="t('darow.selection.recenter')"
                    :aria-label="t('darow.selection.recenter')"
                    @click="recenter"
                >
                    <component :is="icons.Recenter" class="w-3.5 h-3.5" />
                </BaseButton>
                <BaseButton
                    v-if="canEdit"
                    size="xs"
                    variant="primary"
                    :tone="mapEdit.editing.value ? 'solid' : 'soft'"
                    :active="mapEdit.editing.value"
                    :aria-pressed="mapEdit.editing.value"
                    :title="t('darow.mapEdit.title')"
                    @click="mapEdit.toggle"
                >
                    <component :is="icons.Edit" class="w-3.5 h-3.5" />
                    {{ t('darow.mapEdit.toggle') }}
                </BaseButton>
            </PickToolbar>
            <GeometryEditToolbar v-if="mapEdit.editing.value" :editor="editor" topology>
                <span class="mx-0.5 h-5 w-px bg-base-content/15" />
                <BaseButton
                    size="xs"
                    tone="ghost"
                    class="btn-square"
                    :title="t('darow.mapEdit.create')"
                    :aria-label="t('darow.mapEdit.create')"
                    @click="mapEdit.create"
                >
                    <component :is="icons.Add" class="w-4 h-4" />
                </BaseButton>
                <BaseButton
                    size="xs"
                    tone="ghost"
                    class="btn-square"
                    :disabled="!mapEdit.dirty.value"
                    :title="t('darow.mapEdit.cancel')"
                    :aria-label="t('darow.mapEdit.cancel')"
                    @click="mapEdit.cancel"
                >
                    <component :is="icons.Revert" class="w-4 h-4" />
                </BaseButton>
                <BaseButton
                    size="xs"
                    variant="primary"
                    :disabled="!mapEdit.dirty.value"
                    :loading="mapEdit.busy.value"
                    :title="t('darow.mapEdit.save')"
                    :aria-label="t('darow.mapEdit.save')"
                    @click="onSave"
                >
                    <component :is="icons.Save" class="w-4 h-4" />
                </BaseButton>
            </GeometryEditToolbar>
        </div>
        <p
            v-if="mapEdit.editing.value"
            class="absolute bottom-2 left-2 z-20 max-w-[75%] rounded bg-base-100/90 px-2 py-0.5 text-xs"
        >
            <span class="font-semibold">{{ mapEdit.status.value }}</span>
            · {{ editor.hint.value }}
        </p>
    </div>
</template>
