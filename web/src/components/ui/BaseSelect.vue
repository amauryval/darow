<script setup lang="ts">
withDefaults(
    defineProps<{
        modelValue?: string
        options: { value: string; label: string }[]
        size?: 'xs' | 'sm' | 'md'
        // A visible label makes the select a block field, the same shape
        // BaseInput takes in a form; without one it stays an inline control.
        label?: string
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
    <label v-if="label" class="block">
        <span class="mb-1 block text-xs font-medium text-base-content/50">{{ label }}</span>
        <select
            :value="modelValue"
            :aria-label="ariaLabel"
            class="select select-bordered w-full"
            :class="sizes[size]"
            @change="onChange"
        >
            <option v-for="opt in options" :key="opt.value" :value="opt.value">
                {{ opt.label }}
            </option>
        </select>
    </label>
    <select
        v-else
        :value="modelValue"
        :aria-label="ariaLabel"
        class="select select-bordered"
        :class="sizes[size]"
        @change="onChange"
    >
        <option v-for="opt in options" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
    </select>
</template>
