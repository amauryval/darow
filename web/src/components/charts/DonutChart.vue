<script setup lang="ts">
import { computed } from 'vue'
import { pie, arc } from 'd3-shape'

// Presentational: props in, no network, no store. Identity encoding (which
// category a slice belongs to) needs the app's fixed categorical order,
// never a generated hue — see spec/front/spec.md §5. Beyond the three
// available hues, the smallest remaining slices fold into a neutral
// "Autres" bucket instead of inventing a fourth hue.
const props = withDefaults(
    defineProps<{
        data: { label: string; value: number }[]
        valueFormat?: (n: number) => string
    }>(),
    { valueFormat: (n: number) => String(n) }
)

// Fixed order (validated with the dataviz skill's contrast/CVD checks
// against this app's daisyUI `light` theme): primary, secondary, info.
const CATEGORICAL_COLORS = ['var(--color-primary)', 'var(--color-secondary)', 'var(--color-info)']
const OTHER_COLOR = 'var(--color-base-300)'
const OTHER_LABEL = 'Autres'

const RADIUS = 72
const INNER_RADIUS = 40
const GAP_RADIANS = 0.012

const slices = computed(() => {
    const sorted = [...props.data].sort((a, b) => b.value - a.value)
    if (sorted.length <= CATEGORICAL_COLORS.length) {
        return sorted.map((d, i) => ({ ...d, color: CATEGORICAL_COLORS[i] }))
    }
    const kept = sorted.slice(0, CATEGORICAL_COLORS.length - 1)
    const rest = sorted.slice(CATEGORICAL_COLORS.length - 1)
    const folded = { label: OTHER_LABEL, value: rest.reduce((sum, d) => sum + d.value, 0) }
    return [
        ...kept.map((d, i) => ({ ...d, color: CATEGORICAL_COLORS[i] })),
        { ...folded, color: OTHER_COLOR }
    ]
})

const total = computed(() => props.data.reduce((sum, d) => sum + d.value, 0))

const arcGenerator = arc<{ startAngle: number; endAngle: number }>()
    .innerRadius(INNER_RADIUS)
    .outerRadius(RADIUS)
    .cornerRadius(2)
    .padAngle(GAP_RADIANS)

const paths = computed(() => {
    const layout = pie<{ label: string; value: number; color: string }>().value((d) => d.value)(
        slices.value
    )
    return layout.map((arcDatum) => ({
        label: arcDatum.data.label,
        color: arcDatum.data.color,
        value: arcDatum.data.value,
        percent: total.value > 0 ? Math.round((arcDatum.data.value / total.value) * 100) : 0,
        d: arcGenerator(arcDatum) ?? ''
    }))
})
</script>

<template>
    <div v-if="data.length === 0" class="text-sm text-base-content/60">
        Aucune donnée à afficher.
    </div>
    <div v-else class="flex items-center gap-6">
        <svg
            :viewBox="`${-RADIUS} ${-RADIUS} ${RADIUS * 2} ${RADIUS * 2}`"
            width="144"
            height="144"
            role="img"
            :aria-label="paths.map((p) => `${p.label} : ${p.percent}%`).join(', ')"
        >
            <path v-for="slice in paths" :key="slice.label" :d="slice.d" :fill="slice.color">
                <title>
                    {{ slice.label }} : {{ valueFormat(slice.value) }} ({{ slice.percent }}%)
                </title>
            </path>
        </svg>
        <ul class="flex flex-col gap-1.5">
            <li v-for="slice in paths" :key="slice.label" class="flex items-center gap-2 text-sm">
                <span
                    class="h-2.5 w-2.5 shrink-0 rounded-full"
                    :style="{ backgroundColor: slice.color }"
                />
                <span class="text-base-content">{{ slice.label }}</span>
                <span class="text-base-content/60">{{ slice.percent }}%</span>
            </li>
        </ul>
    </div>
</template>
