<script setup lang="ts">
// fraction is 0..1; clamped so a rounding overshoot from onprogress never
// renders past full.
const props = defineProps<{ fraction: number }>()
</script>

<template>
    <progress
        class="progress progress-primary w-full"
        :value="Math.min(1, Math.max(0, props.fraction)) * 100"
        max="100"
    />
</template>
