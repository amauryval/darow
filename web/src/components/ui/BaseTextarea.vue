<script setup lang="ts">
withDefaults(defineProps<{ modelValue?: string; rows?: number; placeholder?: string }>(), {
    rows: 4
})

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

function onInput(event: Event) {
    if (event.target instanceof HTMLTextAreaElement) emit('update:modelValue', event.target.value)
}
</script>

<template>
    <textarea
        :value="modelValue"
        :rows="rows"
        :placeholder="placeholder"
        spellcheck="false"
        class="textarea textarea-bordered w-full resize-y font-mono text-xs"
        @input="onInput"
    />
</template>
