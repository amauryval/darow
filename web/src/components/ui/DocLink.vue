<script setup lang="ts">
import { ExternalLink } from '@lucide/vue'

// An external reference link, styled to sit inline after a caption — used
// next to a free-SQL field to point at the syntax it accepts (DuckDB's
// docs) rather than re-listing it in the app.
defineProps<{ href: string }>()
</script>

<template>
    <a
        :href="href"
        target="_blank"
        rel="noopener noreferrer"
        class="link link-primary inline-flex items-center gap-1 whitespace-nowrap"
    >
        <slot />
        <ExternalLink :size="11" aria-hidden="true" />
    </a>
</template>
