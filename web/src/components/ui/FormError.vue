<script setup lang="ts">
// A form-level error, under the fields or the block it concerns rather than
// a toast: the user still has to act on it, and what they typed is still in
// front of them (§8.4).
defineProps<{ message?: string | null }>()
</script>

<template>
    <p v-if="message" class="mt-2 text-xs text-error" role="alert">{{ message }}</p>
</template>
