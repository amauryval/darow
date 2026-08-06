<script setup lang="ts">
import type { Component } from 'vue'

// A single-number KPI card, on daisyUI's own `stats`/`stat` component
// (https://daisyui.com/components/stat/) rather than a hand-rolled bordered
// box — the primitive daisyUI already ships for exactly this shape. Pure
// display: no fetch, no store.
defineProps<{
    label: string
    value: string
    hint?: string
    icon?: Component
}>()
</script>

<template>
    <div class="stats shadow">
        <div class="stat">
            <div v-if="icon" class="stat-figure text-base-content/40">
                <component :is="icon" :size="20" />
            </div>
            <div class="stat-title">{{ label }}</div>
            <div class="stat-value text-2xl">{{ value }}</div>
            <div v-if="hint" class="stat-desc">{{ hint }}</div>
        </div>
    </div>
</template>
