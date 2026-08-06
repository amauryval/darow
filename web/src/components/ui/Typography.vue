<script setup lang="ts">
import { computed } from 'vue'

type Variant =
    | 'h1'
    | 'h2'
    | 'h3'
    | 'h4'
    | 'body'
    | 'body-sm'
    | 'eyebrow'
    | 'caption'
    | 'code'
    | 'paragraph'

const props = withDefaults(
    defineProps<{
        variant?: Variant
    }>(),
    {
        variant: 'body'
    }
)

const variantMap: Record<Variant, { tag: string; classes: string }> = {
    h1: { tag: 'h1', classes: 'text-3xl font-bold tracking-tight text-base-content' },
    h2: { tag: 'h2', classes: 'text-2xl font-semibold tracking-tight text-base-content' },
    h3: { tag: 'h3', classes: 'text-xl font-semibold text-base-content' },
    h4: { tag: 'h4', classes: 'text-base font-semibold text-base-content' },
    body: { tag: 'p', classes: 'text-base font-normal text-base-content' },
    'body-sm': { tag: 'p', classes: 'text-sm font-normal text-base-content' },
    eyebrow: {
        tag: 'span',
        classes: 'text-xs leading-snug font-semibold uppercase text-base-content/60'
    },
    caption: { tag: 'small', classes: 'text-xs font-normal text-base-content/60' },
    code: {
        tag: 'code',
        classes: 'text-sm font-mono bg-base-200 text-primary px-1.5 py-0.5 rounded'
    },
    paragraph: {
        tag: 'p',
        classes: 'text-sm font-normal text-base-content leading-relaxed text-justify hyphens-auto'
    }
}

const current = computed(() => variantMap[props.variant])
</script>

<template>
    <component :is="current.tag" :class="current.classes">
        <slot />
    </component>
</template>
