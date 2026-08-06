<script setup lang="ts" generic="T extends string">
// daisyUI's tabs/tab/tab-active + the role="tablist"/"tab"/aria-selected
// wiring, carried once rather than hand-rolled at each call site (§0.8,
// §0.10). Generic over the mode's own union (T) so a caller's v-model stays
// typed to its two or three modes rather than widening to a bare string.
defineProps<{ tabs: { value: T; label: string }[]; modelValue: T }>()
defineEmits<{ 'update:modelValue': [value: T] }>()
</script>

<template>
    <div role="tablist" class="tabs tabs-box">
        <button
            v-for="tab in tabs"
            :key="tab.value"
            type="button"
            role="tab"
            class="tab"
            :class="{ 'tab-active': modelValue === tab.value }"
            :aria-selected="modelValue === tab.value"
            @click="$emit('update:modelValue', tab.value)"
        >
            {{ tab.label }}
        </button>
    </div>
</template>
