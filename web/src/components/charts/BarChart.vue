<script setup lang="ts">
import { computed } from 'vue'
import { scaleBand, scaleLinear } from 'd3-scale'
import { max } from 'd3-array'

// Presentational: props in, no network, no store. Horizontal magnitude
// comparison across categories — one hue throughout, since color here
// doesn't encode a second dimension (dataviz skill: a bar-per-category
// chart needs no categorical palette or legend).
const props = withDefaults(
    defineProps<{
        data: { label: string; value: number }[]
        valueFormat?: (n: number) => string
        color?: string
    }>(),
    {
        valueFormat: (n: number) => String(n),
        color: 'var(--color-primary)'
    }
)

const BAR_HEIGHT = 16
const BAR_GAP = 10
const LABEL_WIDTH = 128
const CHART_WIDTH = 320
const PADDING_RIGHT = 48

const chartHeight = computed(() => props.data.length * (BAR_HEIGHT + BAR_GAP))
const plotWidth = CHART_WIDTH - LABEL_WIDTH - PADDING_RIGHT

const yScale = computed(() =>
    scaleBand()
        .domain(props.data.map((d) => d.label))
        .range([0, chartHeight.value])
        .paddingInner(BAR_GAP / (BAR_HEIGHT + BAR_GAP))
)

const xScale = computed(() =>
    scaleLinear()
        .domain([0, max(props.data, (d) => d.value) ?? 0])
        .range([0, plotWidth])
        .nice()
)

const bars = computed(() =>
    props.data.map((d) => ({
        label: d.label,
        value: d.value,
        y: yScale.value(d.label) ?? 0,
        width: Math.max(2, xScale.value(d.value)),
        formatted: props.valueFormat(d.value)
    }))
)
</script>

<template>
    <div v-if="data.length === 0" class="text-sm text-base-content/60">
        Aucune donnée à afficher.
    </div>
    <svg
        v-else
        :viewBox="`0 0 ${CHART_WIDTH} ${chartHeight}`"
        :width="CHART_WIDTH"
        :height="chartHeight"
        class="w-full"
        role="img"
        :aria-label="data.map((d) => `${d.label} : ${valueFormat(d.value)}`).join(', ')"
    >
        <g v-for="bar in bars" :key="bar.label" :transform="`translate(0, ${bar.y})`">
            <text
                :x="LABEL_WIDTH - 8"
                :y="BAR_HEIGHT / 2"
                text-anchor="end"
                dominant-baseline="middle"
                class="fill-base-content/70 text-[10px]"
            >
                {{ bar.label }}
            </text>
            <rect
                :x="LABEL_WIDTH"
                y="0"
                :width="bar.width"
                :height="BAR_HEIGHT"
                rx="4"
                :fill="color"
            >
                <title>{{ bar.label }} : {{ bar.formatted }}</title>
            </rect>
            <text
                :x="LABEL_WIDTH + bar.width + 6"
                :y="BAR_HEIGHT / 2"
                dominant-baseline="middle"
                class="fill-base-content text-[10px] font-medium"
            >
                {{ bar.formatted }}
            </text>
        </g>
    </svg>
</template>
