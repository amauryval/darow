<script setup lang="ts">
// The product's mark: a duck sat on its rows — the idiom the name comes from
// ("get your ducks in a row"), and what the server does to a file. Drawn in
// `currentColor` so the one mark serves wherever it is put; its standalone,
// coloured twin is `public/favicon.svg`, which a browser reads as a file and
// so cannot share this markup.
withDefaults(defineProps<{ size?: number }>(), { size: 18 })
</script>

<template>
    <svg
        :width="size"
        :height="size"
        viewBox="0 0 32 32"
        fill="none"
        aria-hidden="true"
        focusable="false"
    >
        <g fill="currentColor">
            <path d="M2.5 12.5 L8 10.5 L8 17.5 Z" />
            <ellipse cx="14" cy="16.5" rx="8" ry="5" />
            <path
                d="M20 15.5 L22.5 8.5"
                stroke="currentColor"
                stroke-width="6"
                stroke-linecap="round"
            />
            <circle cx="22.5" cy="8" r="4.5" />
            <path d="M26.5 6.8 L31 8.4 L26.5 10.4 Z" />
            <rect x="3" y="24.5" width="26" height="2" rx="1" opacity=".55" />
            <rect x="3" y="28" width="17" height="2" rx="1" opacity=".3" />
        </g>
    </svg>
</template>
