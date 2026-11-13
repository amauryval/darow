<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import type { Map } from 'maplibre-gl'
import type { Geometry } from 'geojson'
import { t } from '@myapplibs/web-platform/i18n'
import ToggleButton from '@myapplibs/web-platform/components/base/ToggleButton.vue'
import FormTextarea from '@myapplibs/web-platform/components/form/FormTextarea.vue'
import GeoJsonMap from '@myapplibs/web-platform/components/map/GeoJsonMap.vue'
import GeometryEditMap from '@myapplibs/web-platform/components/map/GeometryEditMap.vue'
import type { Extent } from '@/darow/api/engine.gen'
import { icons } from '@/darow/icons'
import { collectionOf, geometryText, parseGeometry } from '@/darow/utils/geometry'
import type { ScreenBox } from '@myapplibs/web-platform/components/map/boxSelect'
import { fitExtent } from '@myapplibs/web-platform/components/map/fitPositions'
import {
    addVectorLayers,
    hideVectorFeatures
} from '@myapplibs/web-platform/components/map/vectorTiles'
import { useRowGeometries } from '@/darow/composables/useRowGeometries'
import { useTable } from '@/darow/composables/useTable'
import { TABLE_LAYERS } from '@/darow/utils/tableLayers'

const props = defineProps<{
    disabled: boolean
    tilesUrl: string
    extent?: Extent
    rowId: number | null
}>()
const geometry = defineModel<Geometry | null>({ required: true })

const { name } = useTable()
const exact = useRowGeometries(() => name.value)
const map = shallowRef<Map | null>(null)
const hidden = computed(() => (props.rowId === null ? [] : [props.rowId]))

const snapFeatures = (box: ScreenBox) => (map.value ? exact.near(map.value, box, hidden.value) : [])

const asText = ref(false)
const text = ref(geometryText(geometry.value))

watch(geometry, (next) => {
    if (parseGeometry(text.value) !== next) text.value = geometryText(next)
})

const invalid = computed(() => parseGeometry(text.value) === undefined)

const onText = (next: string) => {
    text.value = next
    const parsed = parseGeometry(next)
    if (parsed !== undefined) geometry.value = parsed
}

const addContext = (target: Map) => {
    addVectorLayers(target, TABLE_LAYERS, props.tilesUrl, [])
    hideVectorFeatures(target, TABLE_LAYERS, hidden.value)
}

const onReady = (target: Map) => {
    map.value = target
    addContext(target)
    if (!geometry.value) fitExtent(target, props.extent)
    else
        target.once(
            'idle',
            () => geometry.value && exact.around(target, geometry.value, hidden.value)
        )
}
</script>

<template>
    <div class="flex flex-col gap-2">
        <div class="flex justify-end">
            <ToggleButton
                :pressed="asText"
                :icon="icons.Code"
                :label="t('darow.geometry.asText')"
                @toggle="asText = !asText"
            />
        </div>
        <FormTextarea
            v-if="asText"
            :model-value="text"
            :legend="t('darow.geometry.geojson')"
            :error="invalid ? t('darow.geometry.invalid') : ''"
            :disabled="props.disabled"
            :rows="8"
            @update:model-value="onText"
        />
        <div class="h-96 rounded-box overflow-hidden">
            <GeoJsonMap v-if="props.disabled" :data="collectionOf(geometry)" />
            <GeometryEditMap
                v-else
                v-model="geometry"
                :snap-features="snapFeatures"
                @map-ready="onReady"
                @style-ready="addContext"
            />
        </div>
    </div>
</template>
