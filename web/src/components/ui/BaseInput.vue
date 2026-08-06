<script setup lang="ts">
withDefaults(
    defineProps<{
        modelValue?: string | number
        type?: string
        inputmode?: 'text' | 'numeric' | 'decimal' | 'email' | 'tel' | 'url' | 'search' | 'none'
        label?: string
        placeholder?: string
        size?: 'xs' | 'sm' | 'md'
        disabled?: boolean
        error?: string
        // For a field whose meaning is carried by its position rather than a
        // visible label (a filter row's value box): the control still needs an
        // accessible name (§8.7). The root element is the <label>, so this has to
        // be bound on the <input> explicitly rather than left to attribute
        // fallthrough.
        ariaLabel?: string
    }>(),
    { type: 'text', size: 'md' }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const sizes = {
    xs: 'input-xs',
    sm: 'input-sm',
    md: ''
}

function onInput(event: Event) {
    if (event.target instanceof HTMLInputElement) emit('update:modelValue', event.target.value)
}
</script>

<template>
    <label class="block">
        <span v-if="label" class="mb-1 block text-xs font-medium text-base-content/50">{{
            label
        }}</span>
        <input
            :value="modelValue"
            :type="type"
            :inputmode="inputmode"
            :placeholder="placeholder"
            :disabled="disabled"
            :aria-label="ariaLabel"
            class="input input-bordered w-full"
            :class="[sizes[size], { 'input-error': error }]"
            @input="onInput"
        />
        <span v-if="error" class="mt-1 block text-xs text-error">{{ error }}</span>
    </label>
</template>
