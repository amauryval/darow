<script setup lang="ts">
withDefaults(
    defineProps<{
        modelValue?: string
        options: { value: string; label: string }[]
        size?: 'xs' | 'sm' | 'md'
        ariaLabel?: string
    }>(),
    { size: 'md' }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const sizes = {
    xs: 'select-xs',
    sm: 'select-sm',
    md: ''
}

function onChange(event: Event) {
    if (event.target instanceof HTMLSelectElement) emit('update:modelValue', event.target.value)
}
</script>

<template>
    <select
        :value="modelValue"
        :aria-label="ariaLabel"
        class="select select-bordered"
        :class="sizes[size]"
        @change="onChange"
    >
        <option v-for="opt in options" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
    </select>
</template>
