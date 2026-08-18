<script setup lang="ts">
import { computed, ref } from 'vue'
import { FileUp, UploadCloud } from '@lucide/vue'
import Modal from '../ui/Modal.vue'
import BaseButton from '../ui/BaseButton.vue'
import ProgressBar from '../ui/ProgressBar.vue'
import Typography from '../ui/Typography.vue'
import type { SourceLimits } from '../../client/types'
import { formatSize } from '../../utils/sourceRows'
import { validateUploadSize } from '../../utils/uploadValidation'

// The running deployment's own upload constraints (GET /sources/limits),
// loaded by the view: stated before the attempt rather than only in an error
// response (§8.5), and never guessed from the default config.yaml.
const props = defineProps<{
    uploading?: boolean
    progress?: number | null
    limits: SourceLimits | null
}>()
const emit = defineEmits<{ close: []; upload: [file: File] }>()

const accept = computed(() => props.limits?.allowedExtensions.join(',') ?? '')
// Absent until the limits land — the server checks both anyway, so the hint
// is skipped rather than shown wrong.
const constraintsLabel = computed(() =>
    props.limits
        ? `${props.limits.allowedExtensions.join(', ')} — ${formatSize(
              props.limits.maxSizeBytes
          )} maximum`
        : null
)

const dragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const error = ref<string | null>(null)

function handleFile(file: File) {
    const message = props.limits ? validateUploadSize(file, props.limits.maxSizeBytes) : null
    error.value = message
    if (!message) emit('upload', file)
}

function onFileSelected(event: Event) {
    const file = event.target instanceof HTMLInputElement ? event.target.files?.[0] : undefined
    // Cleared before handling: picking the same file twice in a row emits no
    // change event otherwise.
    if (fileInput.value) fileInput.value.value = ''
    if (file) handleFile(file)
}

function onDrop(event: DragEvent) {
    dragging.value = false
    const file = event.dataTransfer?.files?.[0]
    if (file) handleFile(file)
}
</script>

<template>
    <Modal title="Importer une source" @close="emit('close')">
        <div
            class="flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 text-center transition-colors"
            :class="dragging ? 'border-primary bg-primary/10' : 'border-base-300 bg-base-200/50'"
            @dragover.prevent="dragging = true"
            @dragleave.prevent="dragging = false"
            @drop.prevent="onDrop"
        >
            <UploadCloud :size="36" class="text-base-content/40" />
            <Typography variant="body-sm">Glissez un fichier ici</Typography>
            <Typography variant="caption">ou</Typography>
            <input
                ref="fileInput"
                type="file"
                class="hidden"
                :accept="accept"
                @change="onFileSelected"
            />
            <BaseButton
                variant="primary"
                size="sm"
                :icon="FileUp"
                :busy="uploading"
                :disabled="uploading"
                @click="fileInput?.click()"
            >
                {{ uploading ? 'Envoi…' : 'Parcourir' }}
            </BaseButton>
            <ProgressBar v-if="typeof progress === 'number'" :fraction="progress" />
            <Typography v-if="constraintsLabel" variant="caption">{{
                constraintsLabel
            }}</Typography>
            <Typography v-if="error" variant="caption" class="text-error">{{ error }}</Typography>
        </div>
    </Modal>
</template>
