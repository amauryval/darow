<script setup lang="ts">
import { nextTick, ref } from 'vue'

const props = withDefaults(
    defineProps<{ modelValue?: string; rows?: number; placeholder?: string }>(),
    { rows: 4 }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const el = ref<HTMLTextAreaElement | null>(null)

function onInput(event: Event) {
    if (event.target instanceof HTMLTextAreaElement) emit('update:modelValue', event.target.value)
}

// Replaces the current selection (or inserts at the caret when there is
// none) with text, then puts the caret right after it — what a reference
// panel's "click to insert" needs, which plain v-model cannot express since
// it only ever replaces the whole value.
async function insertAtCursor(text: string) {
    const node = el.value
    const value = props.modelValue ?? ''
    const start = node?.selectionStart ?? value.length
    const end = node?.selectionEnd ?? value.length
    const next = value.slice(0, start) + text + value.slice(end)
    emit('update:modelValue', next)
    await nextTick()
    node?.focus()
    node?.setSelectionRange(start + text.length, start + text.length)
}

defineExpose({ insertAtCursor })
</script>

<template>
    <textarea
        ref="el"
        :value="modelValue"
        :rows="rows"
        :placeholder="placeholder"
        spellcheck="false"
        class="textarea textarea-bordered w-full resize-y font-mono text-xs"
        @input="onInput"
    />
</template>
