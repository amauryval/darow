<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useFocusTrap } from '../../composables/useFocusTrap'
import BaseButton from './BaseButton.vue'
import Typography from './Typography.vue'

// dirty: a modified form must not be lost by a stray click (§8.3). While
// dirty, a backdrop click is a no-op and Escape asks for confirmation
// instead of closing straight away.
// overflowVisible: daisyUI's modal-box scrolls its own content, which turns
// any out-of-flow layer a field opens (an autocomplete list) into scrollable
// content — a scrollbar appears and the list is clipped at the box's edge.
// A modal short enough not to need scrolling lets it spill over instead.
const props = withDefaults(
    defineProps<{ title: string; wide?: boolean; dirty?: boolean; overflowVisible?: boolean }>(),
    { wide: false, dirty: false, overflowVisible: false }
)
const emit = defineEmits<{ close: []; submit: [] }>()

const box = ref<HTMLElement | null>(null)
useFocusTrap(box)

const confirmingClose = ref(false)

function onBackdropClick() {
    if (props.dirty) return
    emit('close')
}

function requestClose() {
    if (props.dirty && !confirmingClose.value) {
        confirmingClose.value = true
        return
    }
    emit('close')
}

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
        requestClose()
        return
    }
    // Enter submits the modal's form (§8.5) — except inside a textarea, where
    // it means a newline, and on a button, which already handles its own Enter.
    if (
        e.key === 'Enter' &&
        !(e.target instanceof HTMLTextAreaElement) &&
        !(e.target instanceof HTMLButtonElement)
    ) {
        emit('submit')
    }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onUnmounted(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
    <Teleport to="body">
        <div class="modal modal-open">
            <div
                ref="box"
                class="modal-box"
                :class="[
                    props.wide ? 'max-w-2xl' : 'max-w-md',
                    { 'overflow-visible': props.overflowVisible }
                ]"
            >
                <div class="mb-4 flex items-center justify-between">
                    <Typography variant="h4">{{ props.title }}</Typography>
                    <BaseButton
                        variant="ghost"
                        size="xs"
                        square
                        aria-label="Fermer"
                        @click="requestClose"
                        >✕</BaseButton
                    >
                </div>
                <slot />
                <div
                    v-if="confirmingClose"
                    role="alert"
                    class="alert alert-warning alert-soft mt-3 flex items-center justify-between text-xs"
                >
                    <span>Fermer sans enregistrer les modifications ?</span>
                    <div class="flex gap-2">
                        <BaseButton variant="ghost" size="xs" @click="emit('close')"
                            >Fermer</BaseButton
                        >
                        <BaseButton variant="ghost" size="xs" @click="confirmingClose = false"
                            >Continuer</BaseButton
                        >
                    </div>
                </div>
            </div>
            <div class="modal-backdrop" @click="onBackdropClick" />
        </div>
    </Teleport>
</template>
